package i18n

import (
	"embed"
	json "github.com/bytedance/sonic"
	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed localize/*
var fs embed.FS

func GinI18nLocalize() gin.HandlerFunc {
	//dir, _ := os.Getwd()
	return ginI18n.Localize(
		ginI18n.WithBundle(&ginI18n.BundleCfg{
			RootPath:         "./localize/",
			AcceptLanguage:   []language.Tag{language.English, language.Chinese},
			DefaultLanguage:  language.English,
			FormatBundleFile: "json",
			UnmarshalFunc:    json.Unmarshal,
			Loader: &ginI18n.EmbedLoader{
				FS: fs,
			},
		}),
		ginI18n.WithGetLngHandle(
			func(c *gin.Context, defaultLang string) string {
				lang := c.Request.Header.Get("Accept-Language")
				if lang == "" {
					return defaultLang
				}
				return lang
			},
		),
	)
}

func gettext(ctx *gin.Context, param interface{}) string {

	switch param.(type) {
	case string:
		message, err := ginI18n.GetMessage(ctx, param)
		if err != nil {
			return param.(string)
		} else {
			return message
		}
	case map[string]string:
		data := param.(map[string]string)
		message, err := ginI18n.GetMessage(ctx,
			&i18n.LocalizeConfig{
				MessageID: data["id"],
				TemplateData: map[string]string{
					data["key"]: data["value"],
				},
			})
		if err != nil {
			return ""
		} else {
			return message
		}
	default:
		message, err := ginI18n.GetMessage(ctx, param)
		if err != nil {
			return ""
		} else {
			return message
		}
	}

}
