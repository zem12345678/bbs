package validator

import (
	"github.com/go-playground/validator/v10"
	"reflect"
	"regexp"
	"strings"
)

var ValidFuncMap = map[string]func(fl validator.FieldLevel) bool{
	"valid_password": validPassword,
	//"valid_iplist":   validIpList,
	"valid_isBlank": validIsBlank,
	"valid_mobile":  validIsMobile,
	"valid_idCard":  validIdCard,
}

func validPassword(fl validator.FieldLevel) bool {
	fl.Field()
	matched, _ := regexp.Match(`^(?=.*[A-Za-z])(?=.*\d)[A-Za-z\d]{8,}$`, []byte(fl.Field().String()))
	return matched
}

func validIpList(fl validator.FieldLevel) bool {
	if fl.Field().String() == "" {
		return true
	}
	for _, item := range strings.Split(fl.Field().String(), ",") {
		matched, _ := regexp.Match(`\S+`, []byte(item)) //ip_addr
		if !matched {
			return false
		}
	}
	return true
}

func validIsBlank(fl validator.FieldLevel) bool {
	switch fl.Field().Kind() {
	case reflect.String, reflect.Slice:
		return !(fl.Field().Len() == 0)
	case reflect.Bool:
		return !fl.Field().Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fl.Field().Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return fl.Field().Uint() == 0
	case reflect.Float32, reflect.Float64:
		return fl.Field().Float() == 0
	case reflect.Interface, reflect.Ptr:
		return !fl.Field().IsNil()
	}
	return reflect.DeepEqual(fl.Field().Interface(), reflect.Zero(fl.Field().Type()).Interface())
}

func validIsMobile(fl validator.FieldLevel) bool {
	reg := regexp.MustCompile("^1[345789]{1}\\d{9}$")
	return reg.MatchString(fl.Field().String())
}

func validIdCard(fl validator.FieldLevel) bool {
	reg := regexp.MustCompile("(^\\d{15}$)|(^\\d{18}$)|(^\\d{17}(\\d|X|x)$)")
	return reg.MatchString(fl.Field().String())
}
