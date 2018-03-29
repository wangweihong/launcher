package templates

import (
	"ufleet/launcher/code/config"
	"fmt"
	"text/template"
	"log"
)

func GetTemplateWithoutPrefix(filename string) *template.Template {
	t, err := template.ParseFiles(fmt.Sprintf("%s/config/templates/template-%s", config.GDefault.CurrentDir, filename))
	if err != nil {
		log.Fatal(err)
	}
	if t == nil {
		log.Fatalf("get file failed: %s/config/templates/template-%s", config.GDefault.CurrentDir, filename)
	}
	return t
}

func GetTemplate(fileRelativePath string) *template.Template {
	t, err := template.ParseFiles(fmt.Sprintf("%s/%s", config.GDefault.CurrentDir, fileRelativePath))
	if err != nil {
		log.Fatal(err)
	}
	if t == nil {
		log.Fatalf("get file failed: %s/%s", config.GDefault.CurrentDir, fileRelativePath)
	}
	return t
}
