package utils

import (
	"testing"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGetIPField(t *testing.T) {
	Convey("TestGetIPField should return the right field", t, func(){
		src := "192.168.4.106"
		So(GetIPField(src, 1) == "192", ShouldBeTrue)
		So(GetIPField(src, 2) == "168", ShouldBeTrue)
		So(GetIPField(src, 3) == "4", ShouldBeTrue)
		So(GetIPField(src, 4) == "106", ShouldBeTrue)
	})
}

func TestGetValidCh(t *testing.T){
	Convey("TestGetValidCh should return valid characters", t, func(){
		tests := []struct{
			values, expected string
		}{
			{"__Abcdsdfjsdlkf..--",    "Abcdsdfjsdlkf--"},
			{"..--Abcdsdfjsdlkf,ZZzz", "--AbcdsdfjsdlkfZZzz"},
			{"Abcdsdfjsdlkf09ZZzz",    "Abcdsdfjsdlkf09ZZzz"},
		}

		for i := range tests {
			So(GetValidCh(tests[i].values) == tests[i].expected, ShouldBeTrue)
		}
	})
}

func TestGetLowerCh(t *testing.T){
	Convey("TestGetLowerCh should return lower characters", t, func(){
		tests := []struct{
			values, expected string
		}{
			{"AbcDZzZ", "bcz"},
			{"abcDZzz", "abczz"},
		}

		for i := range tests {
			So(GetLowerCh(tests[i].values) == tests[i].expected, ShouldBeTrue)
		}
	})
}