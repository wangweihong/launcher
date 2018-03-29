/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package certs

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net"

	certutil "k8s.io/client-go/util/cert"
	kubeadmconstants "ufleet/launcher/code/utils/certs/constants"
	"ufleet/launcher/code/utils/certs/pkiutil"
	"math/big"
	"regexp"
)

type MasterConfiguration struct {
	API                API
	Networking         Networking

	// APIServerCertSANs sets extra Subject Alternative Names for the API Server signing cert
	APIServerCertSAN string
	// CertificatesDir specifies where to store or look for all required certificates
	CertificatesDir string
}

type API struct {
	// AdvertiseAddress sets the address for the API server to advertise.
	AdvertiseAddress string
	// BindPort sets the secure port for the API Server to bind to
	BindPort int32
}

type Networking struct {
	ServiceSubnet string
	PodSubnet     string
	DNSDomain     string
}

// CreatePKIAssetsJustApiserver 生成apiserver证书及密钥
func CreatePKIAssetsJustApiserver(cfg *MasterConfiguration, hostname string) error {
	pkiDir := cfg.CertificatesDir

	_, svcSubnet, err := net.ParseCIDR(cfg.Networking.ServiceSubnet)
	if err != nil {
		return fmt.Errorf("error parsing CIDR %q: %v", cfg.Networking.ServiceSubnet, err)
	}

	// Build the list of SANs
	altNames := getAltNames([]string{cfg.APIServerCertSAN}, hostname, cfg.Networking.DNSDomain, svcSubnet)
	// Append the address the API Server is advertising
	altNames.IPs = append(altNames.IPs, net.ParseIP(cfg.API.AdvertiseAddress))

	var caCert *x509.Certificate
	var caKey *rsa.PrivateKey

	// 获取ca证书
	caCert, caKey, err = pkiutil.TryLoadCertAndKeyFromDisk(pkiDir, "ca")
	if err != nil {
		return fmt.Errorf("[certificates] can't load ca cert and key from disk [%v]\n", err)
	}
	if !caCert.IsCA {
		return fmt.Errorf("certificate and key could be loaded but the certificate is not a CA")
	}

	// If at least one of them exists, we should try to load them
	// In the case that only one exists, there will most likely be an error anyway

	// The certificate and the key did NOT exist, let's generate them now
	// TODO: Add a test case to verify that this cert has the x509.ExtKeyUsageServerAuth flag
	config := certutil.Config{
		CommonName: "kube-apiserver",
		AltNames:   altNames,
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	apiCert, apiKey, err := pkiutil.NewCertAndKey(caCert, caKey, config)
	if err != nil {
		return fmt.Errorf("failure while creating API server key and certificate [%v]", err)
	}

	if err = pkiutil.WriteCertAndKey(pkiDir, kubeadmconstants.APIServerCertAndKeyBaseName, apiCert, apiKey); err != nil {
		return fmt.Errorf("failure while saving API server certificate and key [%v]", err)
	}
	fmt.Println("[certificates] Generated API server certificate and key.")
	fmt.Printf("[certificates] API Server serving cert is signed for DNS names %v and IPs %v\n", altNames.DNSNames, altNames.IPs)

	return nil
}

// CreatePKIAssetsWithoutApiserver 生成除apiserver外其他kubernetes依赖的证书
func CreatePKIAssetsWithoutApiserver(cfg *MasterConfiguration) error {
	pkiDir := cfg.CertificatesDir

	var caCert *x509.Certificate
	var caKey *rsa.PrivateKey

	// The certificate and the key did NOT exist, let's generate them now
	caCert, caKey, err := pkiutil.NewCertificateAuthority()
	if err != nil {
		return fmt.Errorf("failure while generating CA certificate and key [%v]", err)
	}

	if err = pkiutil.WriteCertAndKey(pkiDir, kubeadmconstants.CACertAndKeyBaseName, caCert, caKey); err != nil {
		return fmt.Errorf("failure while saving CA certificate and key [%v]", err)
	}
	fmt.Println("[certificates] Generated CA certificate and key.")

	// If at least one of them exists, we should try to load them
	// In the case that only one exists, there will most likely be an error anyway

	// The certificate and the key did NOT exist, let's generate them now
	// TODO: Add a test case to verify that this cert has the x509.ExtKeyUsageClientAuth flag
	config := certutil.Config{
		CommonName:   "kube-apiserver-kubelet-client",
		Organization: []string{kubeadmconstants.MastersGroup},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	apiClientCert, apiClientKey, err := pkiutil.NewCertAndKey(caCert, caKey, config)
	if err != nil {
		return fmt.Errorf("failure while creating API server kubelet client key and certificate [%v]", err)
	}

	if err = pkiutil.WriteCertAndKey(pkiDir, kubeadmconstants.APIServerKubeletClientCertAndKeyBaseName, apiClientCert, apiClientKey); err != nil {
		return fmt.Errorf("failure while saving API server kubelet client certificate and key [%v]", err)
	}
	fmt.Println("[certificates] Generated API server kubelet client certificate and key.")

	// The key does NOT exist, let's generate it now
	saTokenSigningKey, err := certutil.NewPrivateKey()
	if err != nil {
		return fmt.Errorf("failure while creating service account token signing key [%v]", err)
	}

	if err = pkiutil.WriteKey(pkiDir, kubeadmconstants.ServiceAccountKeyBaseName, saTokenSigningKey); err != nil {
		return fmt.Errorf("failure while saving service account token signing key [%v]", err)
	}

	if err = pkiutil.WritePublicKey(pkiDir, kubeadmconstants.ServiceAccountKeyBaseName, &saTokenSigningKey.PublicKey); err != nil {
		return fmt.Errorf("failure while saving service account token signing public key [%v]", err)
	}
	fmt.Println("[certificates] Generated service account token signing key and public key.")

	// front proxy CA and client certs are used to secure a front proxy authenticator which is used to assert identity
	// without the client cert, you cannot make use of the front proxy and the kube-aggregator uses this connection
	// so we generate and enable it unconditionally
	// This is a separte CA, so that front proxy identities cannot hit the API and normal client certs cannot be used
	// as front proxies.
	var frontProxyCACert *x509.Certificate
	var frontProxyCAKey *rsa.PrivateKey

	// The certificate and the key did NOT exist, let's generate them now
	frontProxyCACert, frontProxyCAKey, err = pkiutil.NewCertificateAuthority()
	if err != nil {
		return fmt.Errorf("failure while generating front-proxy CA certificate and key [%v]", err)
	}

	if err = pkiutil.WriteCertAndKey(pkiDir, kubeadmconstants.FrontProxyCACertAndKeyBaseName, frontProxyCACert, frontProxyCAKey); err != nil {
		return fmt.Errorf("failure while saving front-proxy CA certificate and key [%v]", err)
	}
	fmt.Println("[certificates] Generated front-proxy CA certificate and key.")

	// At this point we have a front proxy CA signing key.  We can use that create the front proxy client cert if
	// it doesn't already exist.

	// The certificate and the key did NOT exist, let's generate them now
	// TODO: Add a test case to verify that this cert has the x509.ExtKeyUsageClientAuth flag
	config = certutil.Config{
		CommonName: "front-proxy-client",
		Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	apiClientCert, apiClientKey, err = pkiutil.NewCertAndKey(frontProxyCACert, frontProxyCAKey, config)
	if err != nil {
		return fmt.Errorf("failure while creating front-proxy client key and certificate [%v]", err)
	}

	if err = pkiutil.WriteCertAndKey(pkiDir, kubeadmconstants.FrontProxyClientCertAndKeyBaseName, apiClientCert, apiClientKey); err != nil {
		return fmt.Errorf("failure while saving front-proxy client certificate and key [%v]", err)
	}
	fmt.Println("[certificates] Generated front-proxy client certificate and key.")

	fmt.Printf("[certificates] Valid certificates and keys now exist in %q\n", pkiDir)

	return nil
}

// getAltNames builds an AltNames object for the certutil to use when generating the certificates
func getAltNames(cfgAltNames []string, hostname, dnsdomain string, svcSubnet *net.IPNet) certutil.AltNames {
	altNames := certutil.AltNames{
		DNSNames: []string{
			hostname,
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			fmt.Sprintf("kubernetes.default.svc.%s", dnsdomain),
		},
	}

	// Populate IPs/DNSNames from AltNames
	for _, altname := range cfgAltNames {
		if ip := net.ParseIP(altname); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		} else if len(IsDNS1123Subdomain(altname)) == 0 {
			altNames.DNSNames = append(altNames.DNSNames, altname)
		}
	}

	// and lastly, extract the internal IP address for the API server
	internalAPIServerVirtualIP, err := GetIndexedIP(svcSubnet, 1)
	if err != nil {
		fmt.Printf("[certs] WARNING: Unable to get first IP address from the given CIDR (%s): %v\n", svcSubnet.String(), err)
	}
	altNames.IPs = append(altNames.IPs, internalAPIServerVirtualIP)
	return altNames
}


// bigForIP creates a big.Int based on the provided net.IP
func bigForIP(ip net.IP) *big.Int {
	b := ip.To4()
	if b == nil {
		b = ip.To16()
	}
	return big.NewInt(0).SetBytes(b)
}

// addIPOffset adds the provided integer offset to a base big.Int representing a
// net.IP
func addIPOffset(base *big.Int, offset int) net.IP {
	return net.IP(big.NewInt(0).Add(base, big.NewInt(int64(offset))).Bytes())
}

// GetIndexedIP returns a net.IP that is subnet.IP + index in the contiguous IP space.
func GetIndexedIP(subnet *net.IPNet, index int) (net.IP, error) {
	ip := addIPOffset(bigForIP(subnet.IP), index)
	if !subnet.Contains(ip) {
		return nil, fmt.Errorf("can't generate IP with index %d from subnet. subnet too small. subnet: %q", index, subnet)
	}
	return ip, nil
}

// IsDNS1123Subdomain tests for a string that conforms to the definition of a
// subdomain in DNS (RFC 1123).
func IsDNS1123Subdomain(value string) []string {
	var errs []string
	if len(value) > DNS1123SubdomainMaxLength {
		errs = append(errs, MaxLenError(DNS1123SubdomainMaxLength))
	}
	if !dns1123SubdomainRegexp.MatchString(value) {
		errs = append(errs, RegexError(dns1123SubdomainErrorMsg, dns1123SubdomainFmt, "example.com"))
	}
	return errs
}

// MaxLenError returns a string explanation of a "string too long" validation
// failure.
func MaxLenError(length int) string {
	return fmt.Sprintf("must be no more than %d characters", length)
}

const dns1123LabelFmt string = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"

var dns1123SubdomainRegexp = regexp.MustCompile("^" + dns1123SubdomainFmt + "$")

const dns1123SubdomainFmt string = dns1123LabelFmt + "(\\." + dns1123LabelFmt + ")*"
const dns1123SubdomainErrorMsg string = "a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"
const DNS1123SubdomainMaxLength int = 253

// RegexError returns a string explanation of a regex validation failure.
func RegexError(msg string, fmt string, examples ...string) string {
	if len(examples) == 0 {
		return msg + " (regex used for validation is '" + fmt + "')"
	}
	msg += " (e.g. "
	for i := range examples {
		if i > 0 {
			msg += " or "
		}
		msg += "'" + examples[i] + "', "
	}
	msg += "regex used for validation is '" + fmt + "')"
	return msg
}
