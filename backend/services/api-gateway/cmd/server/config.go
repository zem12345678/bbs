package server

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"api-gateway/internal/clients"

	"github.com/spf13/viper"
)

type runtimeConfig struct {
	Service struct {
		Name     string
		HTTPPort int
	}
	PublicBaseURL string
	Auth          struct {
		TokenHeader string
		TokenPrefix string
		JWTSecret   string
		RateLimit   struct {
			RegisterInterval             time.Duration
			RegisterRate                 int
			LoginInterval                time.Duration
			LoginRate                    int
			PasswordResetInterval        time.Duration
			PasswordResetRate            int
			PasswordResetConfirmInterval time.Duration
			PasswordResetConfirmRate     int
			EmailVerificationInterval    time.Duration
			EmailVerificationRate        int
			AdminLoginInterval           time.Duration
			AdminLoginRate               int
		}
	}
	Search struct {
		RateLimit struct {
			ContentInterval time.Duration
			ContentRate     int
			UserInterval    time.Duration
			UserRate        int
		}
	}
	Files struct {
		RateLimit struct {
			UploadInterval time.Duration
			UploadRate     int
		}
	}
	Exports struct {
		RateLimit struct {
			AccountDataInterval time.Duration
			AntennaInterval     time.Duration
			BlockingInterval    time.Duration
			ClipInterval        time.Duration
			FavoriteInterval    time.Duration
			FollowingInterval   time.Duration
			MuteInterval        time.Duration
			NoteInterval        time.Duration
			UserListInterval    time.Duration
		}
	}
	Chat struct {
		RateLimit struct {
			CreateRoomInterval time.Duration
			CreateRoomRate     int
			TicketInterval     time.Duration
			TicketRate         int
			SubscribeInterval  time.Duration
			SubscribeRate      int
			JoinInterval       time.Duration
			JoinRate           int
			SendInterval       time.Duration
			SendRate           int
			ReadInterval       time.Duration
			ReadRate           int
		}
		WebSocket struct {
			MaxConnectionsPerUser int
			MaxConnectionsPerIP   int
		}
	}
	Upstreams clients.Options
}

const (
	localDevJWTSecret                = "bbs-local-dev-secret"
	defaultLocalGatewayPublicBaseURL = "http://127.0.0.1:18080"
	minProductionJWTSecretLength     = 32
	defaultChatJoinInterval          = time.Minute
	defaultChatJoinRate              = 10
	defaultChatCreateRoomInterval    = time.Minute
	defaultChatCreateRoomRate        = 5
	defaultChatTicketInterval        = time.Minute
	defaultChatTicketRate            = 10
	defaultChatSubscribeInterval     = time.Minute
	defaultChatSubscribeRate         = 10
	defaultChatMaxConnectionsPerUser = 5
	defaultChatMaxConnectionsPerIP   = 50
	defaultChatSendInterval          = time.Second
	defaultChatSendRate              = 5
	defaultChatReadInterval          = time.Minute
	defaultChatReadRate              = 60
	defaultAuthRegisterInterval      = time.Hour
	defaultAuthRegisterRate          = 5
	defaultAuthLoginInterval         = time.Minute
	defaultAuthLoginRate             = 10
	defaultAuthResetInterval         = 15 * time.Minute
	defaultAuthResetRate             = 5
	defaultAuthVerifyInterval        = 15 * time.Minute
	defaultAuthVerifyRate            = 5
	defaultAdminLoginInterval        = 15 * time.Minute
	defaultAdminLoginRate            = 5
	defaultSearchContentInterval     = time.Minute
	defaultSearchContentRate         = 60
	defaultSearchUserInterval        = time.Minute
	defaultSearchUserRate            = 10
	defaultFileUploadInterval        = time.Minute
	defaultFileUploadRate            = 10
	defaultAccountDataExportInterval = 72 * time.Hour
	defaultAntennaExportInterval     = time.Hour
	defaultBlockingExportInterval    = time.Hour
	defaultClipExportInterval        = 24 * time.Hour
	defaultFavoriteExportInterval    = 24 * time.Hour
	defaultFollowingExportInterval   = time.Hour
	defaultMuteExportInterval        = time.Hour
	defaultNoteExportInterval        = 24 * time.Hour
	defaultUserListExportInterval    = time.Minute
)

func loadConfig(path string) (*viper.Viper, *runtimeConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, nil, err
	}
	cfg, err := loadRuntimeConfig(v)
	if err != nil {
		return nil, nil, err
	}
	return v, cfg, nil
}

func loadRuntimeConfig(v *viper.Viper) (*runtimeConfig, error) {
	configureEnv(v)
	applyEnvOverrides(v)

	var cfg runtimeConfig
	cfg.Service.Name = firstNonEmpty(v.GetString("service.name"), "bbs-api-gateway")
	cfg.Service.HTTPPort = v.GetInt("service.httpPort")
	if cfg.Service.HTTPPort == 0 {
		cfg.Service.HTTPPort = 8080
	}
	applyLocalPublicBaseURLPortOverride(v, cfg.Service.HTTPPort)
	var err error
	cfg.PublicBaseURL, err = normalizePublicBaseURL(v.GetString("http.publicBaseURL"))
	if err != nil {
		return nil, err
	}
	cfg.Auth.TokenHeader = firstNonEmpty(v.GetString("auth.tokenHeader"), "Authorization")
	cfg.Auth.TokenPrefix = firstNonEmpty(v.GetString("auth.tokenPrefix"), "Bearer")
	cfg.Auth.JWTSecret = firstNonEmpty(v.GetString("auth.jwtSecret"), localDevJWTSecret)
	cfg.Auth.RateLimit.RegisterInterval = positiveDuration(v.GetDuration("auth.rateLimit.registerInterval"), defaultAuthRegisterInterval)
	cfg.Auth.RateLimit.RegisterRate = positiveInt(v.GetInt("auth.rateLimit.registerRate"), defaultAuthRegisterRate)
	cfg.Auth.RateLimit.LoginInterval = positiveDuration(v.GetDuration("auth.rateLimit.loginInterval"), defaultAuthLoginInterval)
	cfg.Auth.RateLimit.LoginRate = positiveInt(v.GetInt("auth.rateLimit.loginRate"), defaultAuthLoginRate)
	cfg.Auth.RateLimit.PasswordResetInterval = positiveDuration(v.GetDuration("auth.rateLimit.passwordResetInterval"), defaultAuthResetInterval)
	cfg.Auth.RateLimit.PasswordResetRate = positiveInt(v.GetInt("auth.rateLimit.passwordResetRate"), defaultAuthResetRate)
	cfg.Auth.RateLimit.PasswordResetConfirmInterval = positiveDuration(v.GetDuration("auth.rateLimit.passwordResetConfirmInterval"), defaultAuthResetInterval)
	cfg.Auth.RateLimit.PasswordResetConfirmRate = positiveInt(v.GetInt("auth.rateLimit.passwordResetConfirmRate"), defaultAuthResetRate)
	cfg.Auth.RateLimit.EmailVerificationInterval = positiveDuration(v.GetDuration("auth.rateLimit.emailVerificationInterval"), defaultAuthVerifyInterval)
	cfg.Auth.RateLimit.EmailVerificationRate = positiveInt(v.GetInt("auth.rateLimit.emailVerificationRate"), defaultAuthVerifyRate)
	cfg.Auth.RateLimit.AdminLoginInterval = positiveDuration(v.GetDuration("auth.rateLimit.adminLoginInterval"), defaultAdminLoginInterval)
	cfg.Auth.RateLimit.AdminLoginRate = positiveInt(v.GetInt("auth.rateLimit.adminLoginRate"), defaultAdminLoginRate)
	cfg.Search.RateLimit.ContentInterval = positiveDuration(v.GetDuration("search.rateLimit.contentInterval"), defaultSearchContentInterval)
	cfg.Search.RateLimit.ContentRate = positiveInt(v.GetInt("search.rateLimit.contentRate"), defaultSearchContentRate)
	cfg.Search.RateLimit.UserInterval = positiveDuration(v.GetDuration("search.rateLimit.userInterval"), defaultSearchUserInterval)
	cfg.Search.RateLimit.UserRate = positiveInt(v.GetInt("search.rateLimit.userRate"), defaultSearchUserRate)
	cfg.Files.RateLimit.UploadInterval = positiveDuration(v.GetDuration("files.rateLimit.uploadInterval"), defaultFileUploadInterval)
	cfg.Files.RateLimit.UploadRate = positiveInt(v.GetInt("files.rateLimit.uploadRate"), defaultFileUploadRate)
	cfg.Exports.RateLimit.AccountDataInterval = positiveDuration(v.GetDuration("exports.rateLimit.accountDataInterval"), defaultAccountDataExportInterval)
	cfg.Exports.RateLimit.AntennaInterval = positiveDuration(v.GetDuration("exports.rateLimit.antennaInterval"), defaultAntennaExportInterval)
	cfg.Exports.RateLimit.BlockingInterval = positiveDuration(v.GetDuration("exports.rateLimit.blockingInterval"), defaultBlockingExportInterval)
	cfg.Exports.RateLimit.ClipInterval = positiveDuration(v.GetDuration("exports.rateLimit.clipInterval"), defaultClipExportInterval)
	cfg.Exports.RateLimit.FavoriteInterval = positiveDuration(v.GetDuration("exports.rateLimit.favoriteInterval"), defaultFavoriteExportInterval)
	cfg.Exports.RateLimit.FollowingInterval = positiveDuration(v.GetDuration("exports.rateLimit.followingInterval"), defaultFollowingExportInterval)
	cfg.Exports.RateLimit.MuteInterval = positiveDuration(v.GetDuration("exports.rateLimit.muteInterval"), defaultMuteExportInterval)
	cfg.Exports.RateLimit.NoteInterval = positiveDuration(v.GetDuration("exports.rateLimit.noteInterval"), defaultNoteExportInterval)
	cfg.Exports.RateLimit.UserListInterval = positiveDuration(v.GetDuration("exports.rateLimit.userListInterval"), defaultUserListExportInterval)
	cfg.Chat.RateLimit.TicketInterval = positiveDuration(v.GetDuration("chat.rateLimit.ticketInterval"), defaultChatTicketInterval)
	cfg.Chat.RateLimit.TicketRate = positiveInt(v.GetInt("chat.rateLimit.ticketRate"), defaultChatTicketRate)
	cfg.Chat.RateLimit.CreateRoomInterval = positiveDuration(v.GetDuration("chat.rateLimit.createRoomInterval"), defaultChatCreateRoomInterval)
	cfg.Chat.RateLimit.CreateRoomRate = positiveInt(v.GetInt("chat.rateLimit.createRoomRate"), defaultChatCreateRoomRate)
	cfg.Chat.RateLimit.SubscribeInterval = positiveDuration(v.GetDuration("chat.rateLimit.subscribeInterval"), defaultChatSubscribeInterval)
	cfg.Chat.RateLimit.SubscribeRate = positiveInt(v.GetInt("chat.rateLimit.subscribeRate"), defaultChatSubscribeRate)
	cfg.Chat.RateLimit.JoinInterval = positiveDuration(v.GetDuration("chat.rateLimit.joinInterval"), defaultChatJoinInterval)
	cfg.Chat.RateLimit.JoinRate = positiveInt(v.GetInt("chat.rateLimit.joinRate"), defaultChatJoinRate)
	cfg.Chat.RateLimit.SendInterval = positiveDuration(v.GetDuration("chat.rateLimit.sendInterval"), defaultChatSendInterval)
	cfg.Chat.RateLimit.SendRate = positiveInt(v.GetInt("chat.rateLimit.sendRate"), defaultChatSendRate)
	cfg.Chat.RateLimit.ReadInterval = positiveDuration(v.GetDuration("chat.rateLimit.readInterval"), defaultChatReadInterval)
	cfg.Chat.RateLimit.ReadRate = positiveInt(v.GetInt("chat.rateLimit.readRate"), defaultChatReadRate)
	cfg.Chat.WebSocket.MaxConnectionsPerUser = positiveInt(v.GetInt("chat.websocket.maxConnectionsPerUser"), defaultChatMaxConnectionsPerUser)
	cfg.Chat.WebSocket.MaxConnectionsPerIP = positiveInt(v.GetInt("chat.websocket.maxConnectionsPerIP"), defaultChatMaxConnectionsPerIP)
	cfg.Upstreams = clients.NewOptions(v)
	if isProductionEnvironment(v.GetString("trace.env")) {
		if err := validateProductionSecurityConfig(cfg.Auth.JWTSecret, cfg.Upstreams.AdminInternalAuthToken, cfg.Upstreams.ChatInternalAuthToken, cfg.Upstreams.UserInternalAuthToken, cfg.Upstreams.MallInternalAuthToken, cfg.Upstreams.CreditInternalAuthToken, cfg.Upstreams.FileInternalAuthToken, cfg.Upstreams.FeedInternalAuthToken, cfg.Upstreams.NotificationInternalAuthToken, cfg.Upstreams.ReactionInternalAuthToken, cfg.Upstreams.SearchInternalAuthToken, cfg.Upstreams.CommentInternalAuthToken, cfg.Upstreams.ContentInternalAuthToken, v.GetStringSlice("cors.allowedOrigins")); err != nil {
			return nil, err
		}
		if err := validateProductionInternalTransport(v, cfg.Upstreams); err != nil {
			return nil, err
		}
		if cfg.PublicBaseURL == "" {
			return nil, fmt.Errorf("http.publicBaseURL is required in production")
		}
	}

	v.Set("service.name", cfg.Service.Name)
	v.Set("service.httpPort", cfg.Service.HTTPPort)
	v.Set("http.publicBaseURL", cfg.PublicBaseURL)
	v.Set("auth.tokenHeader", cfg.Auth.TokenHeader)
	v.Set("auth.tokenPrefix", cfg.Auth.TokenPrefix)
	v.Set("auth.jwtSecret", cfg.Auth.JWTSecret)
	v.Set("auth.rateLimit.registerInterval", cfg.Auth.RateLimit.RegisterInterval)
	v.Set("auth.rateLimit.registerRate", cfg.Auth.RateLimit.RegisterRate)
	v.Set("auth.rateLimit.loginInterval", cfg.Auth.RateLimit.LoginInterval)
	v.Set("auth.rateLimit.loginRate", cfg.Auth.RateLimit.LoginRate)
	v.Set("auth.rateLimit.passwordResetInterval", cfg.Auth.RateLimit.PasswordResetInterval)
	v.Set("auth.rateLimit.passwordResetRate", cfg.Auth.RateLimit.PasswordResetRate)
	v.Set("auth.rateLimit.passwordResetConfirmInterval", cfg.Auth.RateLimit.PasswordResetConfirmInterval)
	v.Set("auth.rateLimit.passwordResetConfirmRate", cfg.Auth.RateLimit.PasswordResetConfirmRate)
	v.Set("auth.rateLimit.emailVerificationInterval", cfg.Auth.RateLimit.EmailVerificationInterval)
	v.Set("auth.rateLimit.emailVerificationRate", cfg.Auth.RateLimit.EmailVerificationRate)
	v.Set("auth.rateLimit.adminLoginInterval", cfg.Auth.RateLimit.AdminLoginInterval)
	v.Set("auth.rateLimit.adminLoginRate", cfg.Auth.RateLimit.AdminLoginRate)
	v.Set("search.rateLimit.contentInterval", cfg.Search.RateLimit.ContentInterval)
	v.Set("search.rateLimit.contentRate", cfg.Search.RateLimit.ContentRate)
	v.Set("search.rateLimit.userInterval", cfg.Search.RateLimit.UserInterval)
	v.Set("search.rateLimit.userRate", cfg.Search.RateLimit.UserRate)
	v.Set("files.rateLimit.uploadInterval", cfg.Files.RateLimit.UploadInterval)
	v.Set("files.rateLimit.uploadRate", cfg.Files.RateLimit.UploadRate)
	v.Set("exports.rateLimit.accountDataInterval", cfg.Exports.RateLimit.AccountDataInterval)
	v.Set("exports.rateLimit.antennaInterval", cfg.Exports.RateLimit.AntennaInterval)
	v.Set("exports.rateLimit.blockingInterval", cfg.Exports.RateLimit.BlockingInterval)
	v.Set("exports.rateLimit.clipInterval", cfg.Exports.RateLimit.ClipInterval)
	v.Set("exports.rateLimit.favoriteInterval", cfg.Exports.RateLimit.FavoriteInterval)
	v.Set("exports.rateLimit.followingInterval", cfg.Exports.RateLimit.FollowingInterval)
	v.Set("exports.rateLimit.muteInterval", cfg.Exports.RateLimit.MuteInterval)
	v.Set("exports.rateLimit.noteInterval", cfg.Exports.RateLimit.NoteInterval)
	v.Set("exports.rateLimit.userListInterval", cfg.Exports.RateLimit.UserListInterval)
	v.Set("chat.rateLimit.ticketInterval", cfg.Chat.RateLimit.TicketInterval)
	v.Set("chat.rateLimit.ticketRate", cfg.Chat.RateLimit.TicketRate)
	v.Set("chat.rateLimit.createRoomInterval", cfg.Chat.RateLimit.CreateRoomInterval)
	v.Set("chat.rateLimit.createRoomRate", cfg.Chat.RateLimit.CreateRoomRate)
	v.Set("chat.rateLimit.subscribeInterval", cfg.Chat.RateLimit.SubscribeInterval)
	v.Set("chat.rateLimit.subscribeRate", cfg.Chat.RateLimit.SubscribeRate)
	v.Set("chat.rateLimit.joinInterval", cfg.Chat.RateLimit.JoinInterval)
	v.Set("chat.rateLimit.joinRate", cfg.Chat.RateLimit.JoinRate)
	v.Set("chat.rateLimit.sendInterval", cfg.Chat.RateLimit.SendInterval)
	v.Set("chat.rateLimit.sendRate", cfg.Chat.RateLimit.SendRate)
	v.Set("chat.rateLimit.readInterval", cfg.Chat.RateLimit.ReadInterval)
	v.Set("chat.rateLimit.readRate", cfg.Chat.RateLimit.ReadRate)
	v.Set("chat.websocket.maxConnectionsPerUser", cfg.Chat.WebSocket.MaxConnectionsPerUser)
	v.Set("chat.websocket.maxConnectionsPerIP", cfg.Chat.WebSocket.MaxConnectionsPerIP)
	v.Set("upstreams.chatInternalAuthToken", cfg.Upstreams.ChatInternalAuthToken)
	v.Set("upstreams.adminInternalAuthToken", cfg.Upstreams.AdminInternalAuthToken)
	v.Set("upstreams.userInternalAuthToken", cfg.Upstreams.UserInternalAuthToken)
	v.Set("upstreams.mallInternalAuthToken", cfg.Upstreams.MallInternalAuthToken)
	v.Set("upstreams.creditInternalAuthToken", cfg.Upstreams.CreditInternalAuthToken)
	v.Set("upstreams.fileInternalAuthToken", cfg.Upstreams.FileInternalAuthToken)
	v.Set("upstreams.feedInternalAuthToken", cfg.Upstreams.FeedInternalAuthToken)
	v.Set("upstreams.notificationInternalAuthToken", cfg.Upstreams.NotificationInternalAuthToken)
	v.Set("upstreams.reactionInternalAuthToken", cfg.Upstreams.ReactionInternalAuthToken)
	v.Set("upstreams.searchInternalAuthToken", cfg.Upstreams.SearchInternalAuthToken)
	v.Set("upstreams.commentInternalAuthToken", cfg.Upstreams.CommentInternalAuthToken)
	v.Set("upstreams.contentInternalAuthToken", cfg.Upstreams.ContentInternalAuthToken)
	v.Set("upstreams.adminInternalAuthSecure", cfg.Upstreams.AdminInternalAuthSecure)
	v.Set("upstreams.userInternalAuthSecure", cfg.Upstreams.UserInternalAuthSecure)
	v.Set("upstreams.mallInternalAuthSecure", cfg.Upstreams.MallInternalAuthSecure)
	v.Set("upstreams.creditInternalAuthSecure", cfg.Upstreams.CreditInternalAuthSecure)
	v.Set("upstreams.fileInternalAuthSecure", cfg.Upstreams.FileInternalAuthSecure)
	v.Set("upstreams.chatInternalAuthSecure", cfg.Upstreams.ChatInternalAuthSecure)
	return &cfg, nil
}

func positiveDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func positiveInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func normalizePublicBaseURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("http.publicBaseURL must be an absolute HTTP(S) URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("http.publicBaseURL must contain only scheme and host")
	}
	return strings.ToLower(parsed.Scheme) + "://" + parsed.Host, nil
}

func validateProductionSecurityConfig(jwtSecret string, adminInternalAuthToken string, chatInternalAuthToken string, userInternalAuthToken string, mallInternalAuthToken string, creditInternalAuthToken string, fileInternalAuthToken string, feedInternalAuthToken string, notificationInternalAuthToken string, reactionInternalAuthToken string, searchInternalAuthToken string, commentInternalAuthToken string, contentInternalAuthToken string, corsAllowedOrigins []string) error {
	secret := strings.TrimSpace(jwtSecret)
	if secret == "" || secret == localDevJWTSecret {
		return fmt.Errorf("auth.jwtSecret must be set to a non-default value in production")
	}
	if len(secret) < minProductionJWTSecretLength {
		return fmt.Errorf("auth.jwtSecret must be at least %d characters in production", minProductionJWTSecretLength)
	}
	if err := validateProductionAdminInternalAuthToken(adminInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionChatInternalAuthToken(chatInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionUserInternalAuthToken(userInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionMallInternalAuthToken(mallInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionCreditInternalAuthToken(creditInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionFileInternalAuthToken(fileInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionFeedInternalAuthToken(feedInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionNotificationInternalAuthToken(notificationInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionReactionInternalAuthToken(reactionInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionSearchInternalAuthToken(searchInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionCommentInternalAuthToken(commentInternalAuthToken); err != nil {
		return err
	}
	if err := validateProductionContentInternalAuthToken(contentInternalAuthToken); err != nil {
		return err
	}
	if len(corsAllowedOrigins) == 0 {
		return fmt.Errorf("cors.allowedOrigins must be explicitly set in production")
	}
	for _, origin := range corsAllowedOrigins {
		if err := validateProductionCORSOrigin(origin); err != nil {
			return err
		}
	}
	return nil
}

func validateProductionAdminInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevAdminInternalAuthToken {
		return fmt.Errorf("upstreams.adminInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionAdminInternalAuthBytes {
		return fmt.Errorf("upstreams.adminInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionAdminInternalAuthBytes)
	}
	return nil
}

func validateProductionChatInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevChatInternalAuthToken {
		return fmt.Errorf("upstreams.chatInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionChatInternalAuthBytes {
		return fmt.Errorf("upstreams.chatInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionChatInternalAuthBytes)
	}
	return nil
}

func validateProductionUserInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevUserInternalAuthToken {
		return fmt.Errorf("upstreams.userInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionUserInternalAuthBytes {
		return fmt.Errorf("upstreams.userInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionUserInternalAuthBytes)
	}
	return nil
}

func validateProductionMallInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevMallInternalAuthToken {
		return fmt.Errorf("upstreams.mallInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionMallInternalAuthBytes {
		return fmt.Errorf("upstreams.mallInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionMallInternalAuthBytes)
	}
	return nil
}

func validateProductionCreditInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevCreditInternalAuthToken {
		return fmt.Errorf("upstreams.creditInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionCreditInternalAuthBytes {
		return fmt.Errorf("upstreams.creditInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionCreditInternalAuthBytes)
	}
	return nil
}

func validateProductionFileInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevFileInternalAuthToken {
		return fmt.Errorf("upstreams.fileInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionFileInternalAuthBytes {
		return fmt.Errorf("upstreams.fileInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionFileInternalAuthBytes)
	}
	return nil
}

func validateProductionFeedInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevFeedInternalAuthToken {
		return fmt.Errorf("upstreams.feedInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionFeedInternalAuthBytes {
		return fmt.Errorf("upstreams.feedInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionFeedInternalAuthBytes)
	}
	return nil
}

func validateProductionNotificationInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevNotificationInternalAuthToken {
		return fmt.Errorf("upstreams.notificationInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionNotificationInternalAuthBytes {
		return fmt.Errorf("upstreams.notificationInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionNotificationInternalAuthBytes)
	}
	return nil
}

func validateProductionReactionInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevReactionInternalAuthToken {
		return fmt.Errorf("upstreams.reactionInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionReactionInternalAuthBytes {
		return fmt.Errorf("upstreams.reactionInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionReactionInternalAuthBytes)
	}
	return nil
}

func validateProductionSearchInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevSearchInternalAuthToken {
		return fmt.Errorf("upstreams.searchInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionSearchInternalAuthBytes {
		return fmt.Errorf("upstreams.searchInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionSearchInternalAuthBytes)
	}
	return nil
}

func validateProductionCommentInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevCommentInternalAuthToken {
		return fmt.Errorf("upstreams.commentInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionCommentInternalAuthBytes {
		return fmt.Errorf("upstreams.commentInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionCommentInternalAuthBytes)
	}
	return nil
}

func validateProductionContentInternalAuthToken(value string) error {
	token := strings.TrimSpace(value)
	if token == "" || token == clients.LocalDevContentInternalAuthToken {
		return fmt.Errorf("upstreams.contentInternalAuthToken must be set to a non-default value in production")
	}
	if len([]byte(token)) < clients.MinimumProductionContentInternalAuthBytes {
		return fmt.Errorf("upstreams.contentInternalAuthToken must be at least %d bytes in production", clients.MinimumProductionContentInternalAuthBytes)
	}
	return nil
}

// validateProductionInternalTransport keeps the gateway's protected upstream
// connections aligned with the Admin and Chat services, both of which require
// mTLS in production. Other protected upstreams can remain plaintext until
// their servers are migrated independently.
func validateProductionInternalTransport(v *viper.Viper, upstreams clients.Options) error {
	if !upstreams.AdminInternalAuthSecure {
		return fmt.Errorf("upstreams.adminInternalAuthSecure must be true in production")
	}
	if !upstreams.ChatInternalAuthSecure {
		return fmt.Errorf("upstreams.chatInternalAuthSecure must be true in production")
	}
	for _, upstream := range []struct {
		name    string
		enabled bool
	}{
		{name: "upstreams.userInternalAuthSecure", enabled: upstreams.UserInternalAuthSecure},
		{name: "upstreams.mallInternalAuthSecure", enabled: upstreams.MallInternalAuthSecure},
		{name: "upstreams.creditInternalAuthSecure", enabled: upstreams.CreditInternalAuthSecure},
		{name: "upstreams.fileInternalAuthSecure", enabled: upstreams.FileInternalAuthSecure},
	} {
		if upstream.enabled {
			return fmt.Errorf("%s must remain false until that upstream's mTLS server rollout is complete", upstream.name)
		}
	}
	for _, key := range []string{"grpc.client.tls.caFile", "grpc.client.tls.certFile", "grpc.client.tls.keyFile"} {
		if strings.TrimSpace(v.GetString(key)) == "" {
			return fmt.Errorf("%s is required for production mTLS upstreams", key)
		}
	}
	if strings.TrimSpace(v.GetString("grpc.client.tls.serverName")) != "" {
		return fmt.Errorf("grpc.client.tls.serverName must be empty so each mTLS upstream verifies its own service DNS name")
	}
	return nil
}

func validateProductionCORSOrigin(origin string) error {
	trimmed := strings.TrimSpace(origin)
	if trimmed == "" {
		return fmt.Errorf("cors.allowedOrigins contains an empty origin in production")
	}
	if trimmed == "*" {
		return fmt.Errorf("cors.allowedOrigins must not contain wildcard origins in production")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("cors.allowedOrigins contains invalid origin %q in production", origin)
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return fmt.Errorf("cors.allowedOrigins must use https in production: %q", origin)
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "0.0.0.0" || host == "::1" || strings.HasPrefix(host, "127.") {
		return fmt.Errorf("cors.allowedOrigins must not contain local origins in production: %q", origin)
	}
	return nil
}

func configureEnv(v *viper.Viper) {
	v.SetEnvPrefix("BBS_GATEWAY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	bindEnv(v, "app.name", "BBS_GATEWAY_APP_NAME")
	bindEnv(v, "service.name", "BBS_GATEWAY_SERVICE_NAME")
	bindEnv(v, "service.httpPort", "BBS_GATEWAY_SERVICE_HTTP_PORT")
	bindEnv(v, "grpc.client.secure", "BBS_GATEWAY_GRPC_CLIENT_SECURE")
	bindEnv(v, "grpc.client.tls.caFile", "BBS_GATEWAY_GRPC_CLIENT_TLS_CA_FILE")
	bindEnv(v, "grpc.client.tls.certFile", "BBS_GATEWAY_GRPC_CLIENT_TLS_CERT_FILE")
	bindEnv(v, "grpc.client.tls.keyFile", "BBS_GATEWAY_GRPC_CLIENT_TLS_KEY_FILE")
	bindEnv(v, "grpc.client.tls.serverName", "BBS_GATEWAY_GRPC_CLIENT_TLS_SERVER_NAME")
	bindEnv(v, "auth.tokenHeader", "BBS_GATEWAY_AUTH_TOKEN_HEADER")
	bindEnv(v, "auth.tokenPrefix", "BBS_GATEWAY_AUTH_TOKEN_PREFIX")
	bindEnv(v, "auth.jwtSecret", "BBS_GATEWAY_AUTH_JWT_SECRET")
	bindEnv(v, "auth.rateLimit.registerInterval", "BBS_GATEWAY_AUTH_RATE_LIMIT_REGISTER_INTERVAL")
	bindEnv(v, "auth.rateLimit.registerRate", "BBS_GATEWAY_AUTH_RATE_LIMIT_REGISTER_RATE")
	bindEnv(v, "auth.rateLimit.loginInterval", "BBS_GATEWAY_AUTH_RATE_LIMIT_LOGIN_INTERVAL")
	bindEnv(v, "auth.rateLimit.loginRate", "BBS_GATEWAY_AUTH_RATE_LIMIT_LOGIN_RATE")
	bindEnv(v, "auth.rateLimit.passwordResetInterval", "BBS_GATEWAY_AUTH_RATE_LIMIT_PASSWORD_RESET_INTERVAL")
	bindEnv(v, "auth.rateLimit.passwordResetRate", "BBS_GATEWAY_AUTH_RATE_LIMIT_PASSWORD_RESET_RATE")
	bindEnv(v, "auth.rateLimit.passwordResetConfirmInterval", "BBS_GATEWAY_AUTH_RATE_LIMIT_PASSWORD_RESET_CONFIRM_INTERVAL")
	bindEnv(v, "auth.rateLimit.passwordResetConfirmRate", "BBS_GATEWAY_AUTH_RATE_LIMIT_PASSWORD_RESET_CONFIRM_RATE")
	bindEnv(v, "auth.rateLimit.emailVerificationInterval", "BBS_GATEWAY_AUTH_RATE_LIMIT_EMAIL_VERIFICATION_INTERVAL")
	bindEnv(v, "auth.rateLimit.emailVerificationRate", "BBS_GATEWAY_AUTH_RATE_LIMIT_EMAIL_VERIFICATION_RATE")
	bindEnv(v, "auth.rateLimit.adminLoginInterval", "BBS_GATEWAY_AUTH_RATE_LIMIT_ADMIN_LOGIN_INTERVAL")
	bindEnv(v, "auth.rateLimit.adminLoginRate", "BBS_GATEWAY_AUTH_RATE_LIMIT_ADMIN_LOGIN_RATE")
	bindEnv(v, "search.rateLimit.contentInterval", "BBS_GATEWAY_SEARCH_RATE_LIMIT_CONTENT_INTERVAL")
	bindEnv(v, "search.rateLimit.contentRate", "BBS_GATEWAY_SEARCH_RATE_LIMIT_CONTENT_RATE")
	bindEnv(v, "search.rateLimit.userInterval", "BBS_GATEWAY_SEARCH_RATE_LIMIT_USER_INTERVAL")
	bindEnv(v, "search.rateLimit.userRate", "BBS_GATEWAY_SEARCH_RATE_LIMIT_USER_RATE")
	bindEnv(v, "files.rateLimit.uploadInterval", "BBS_GATEWAY_FILES_RATE_LIMIT_UPLOAD_INTERVAL")
	bindEnv(v, "files.rateLimit.uploadRate", "BBS_GATEWAY_FILES_RATE_LIMIT_UPLOAD_RATE")
	bindEnv(v, "exports.rateLimit.accountDataInterval", "BBS_GATEWAY_EXPORTS_RATE_LIMIT_ACCOUNT_DATA_INTERVAL")
	bindEnv(v, "exports.rateLimit.antennaInterval", "BBS_GATEWAY_EXPORTS_RATE_LIMIT_ANTENNA_INTERVAL")
	bindEnv(v, "exports.rateLimit.blockingInterval", "BBS_GATEWAY_EXPORTS_RATE_LIMIT_BLOCKING_INTERVAL")
	bindEnv(v, "exports.rateLimit.clipInterval", "BBS_GATEWAY_EXPORTS_RATE_LIMIT_CLIP_INTERVAL")
	bindEnv(v, "exports.rateLimit.favoriteInterval", "BBS_GATEWAY_EXPORTS_RATE_LIMIT_FAVORITE_INTERVAL")
	bindEnv(v, "exports.rateLimit.followingInterval", "BBS_GATEWAY_EXPORTS_RATE_LIMIT_FOLLOWING_INTERVAL")
	bindEnv(v, "exports.rateLimit.muteInterval", "BBS_GATEWAY_EXPORTS_RATE_LIMIT_MUTE_INTERVAL")
	bindEnv(v, "exports.rateLimit.noteInterval", "BBS_GATEWAY_EXPORTS_RATE_LIMIT_NOTE_INTERVAL")
	bindEnv(v, "exports.rateLimit.userListInterval", "BBS_GATEWAY_EXPORTS_RATE_LIMIT_USER_LIST_INTERVAL")
	bindEnv(v, "http.trustedProxies", "BBS_GATEWAY_HTTP_TRUSTED_PROXIES")
	bindEnv(v, "http.host", "BBS_GATEWAY_HTTP_HOST")
	bindEnv(v, "http.publicBaseURL", "BBS_GATEWAY_HTTP_PUBLIC_BASE_URL")
	bindEnv(v, "http.pprofEnabled", "BBS_GATEWAY_HTTP_PPROF_ENABLED")
	bindEnv(v, "log.filename", "BBS_GATEWAY_LOG_FILENAME")
	bindEnv(v, "log.level", "BBS_GATEWAY_LOG_LEVEL")
	bindEnv(v, "log.stdout", "BBS_GATEWAY_LOG_STDOUT")
	bindEnv(v, "trace.grpcEndpoint", "BBS_GATEWAY_TRACE_GRPC_ENDPOINT")
	bindEnv(v, "trace.serviceName", "BBS_GATEWAY_TRACE_SERVICE_NAME")
	bindEnv(v, "trace.version", "BBS_GATEWAY_TRACE_VERSION")
	bindEnv(v, "trace.env", "BBS_GATEWAY_TRACE_ENV")
	bindEnv(v, "cors.allowedOrigins", "BBS_GATEWAY_CORS_ALLOWED_ORIGINS")
	bindEnv(v, "upstreams.admin", "BBS_GATEWAY_UPSTREAMS_ADMIN")
	bindEnv(v, "upstreams.adminInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_ADMIN_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.adminInternalAuthSecure", "BBS_GATEWAY_UPSTREAMS_ADMIN_INTERNAL_AUTH_SECURE")
	bindEnv(v, "upstreams.user", "BBS_GATEWAY_UPSTREAMS_USER")
	bindEnv(v, "upstreams.content", "BBS_GATEWAY_UPSTREAMS_CONTENT")
	bindEnv(v, "upstreams.contentInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_CONTENT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.comment", "BBS_GATEWAY_UPSTREAMS_COMMENT")
	bindEnv(v, "upstreams.commentInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_COMMENT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.reaction", "BBS_GATEWAY_UPSTREAMS_REACTION")
	bindEnv(v, "upstreams.reactionInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_REACTION_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.search", "BBS_GATEWAY_UPSTREAMS_SEARCH")
	bindEnv(v, "upstreams.searchInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_SEARCH_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.feed", "BBS_GATEWAY_UPSTREAMS_FEED")
	bindEnv(v, "upstreams.feedInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_FEED_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.credit", "BBS_GATEWAY_UPSTREAMS_CREDIT")
	bindEnv(v, "upstreams.creditInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_CREDIT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.creditInternalAuthSecure", "BBS_GATEWAY_UPSTREAMS_CREDIT_INTERNAL_AUTH_SECURE")
	bindEnv(v, "upstreams.mall", "BBS_GATEWAY_UPSTREAMS_MALL")
	bindEnv(v, "upstreams.mallInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_MALL_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.mallInternalAuthSecure", "BBS_GATEWAY_UPSTREAMS_MALL_INTERNAL_AUTH_SECURE")
	bindEnv(v, "upstreams.notification", "BBS_GATEWAY_UPSTREAMS_NOTIFICATION")
	bindEnv(v, "upstreams.notificationInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_NOTIFICATION_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.file", "BBS_GATEWAY_UPSTREAMS_FILE")
	bindEnv(v, "upstreams.fileInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_FILE_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.fileInternalAuthSecure", "BBS_GATEWAY_UPSTREAMS_FILE_INTERNAL_AUTH_SECURE")
	bindEnv(v, "upstreams.chat", "BBS_GATEWAY_UPSTREAMS_CHAT")
	bindEnv(v, "upstreams.chatInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_CHAT_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.chatInternalAuthSecure", "BBS_GATEWAY_UPSTREAMS_CHAT_INTERNAL_AUTH_SECURE")
	bindEnv(v, "upstreams.userInternalAuthToken", "BBS_GATEWAY_UPSTREAMS_USER_INTERNAL_AUTH_TOKEN")
	bindEnv(v, "upstreams.userInternalAuthSecure", "BBS_GATEWAY_UPSTREAMS_USER_INTERNAL_AUTH_SECURE")
	bindEnv(v, "storage.endpoint", "BBS_GATEWAY_STORAGE_ENDPOINT")
	bindEnv(v, "storage.bucket", "BBS_GATEWAY_STORAGE_BUCKET")
	bindEnv(v, "storage.accessKey", "BBS_GATEWAY_STORAGE_ACCESS_KEY")
	bindEnv(v, "storage.secretKey", "BBS_GATEWAY_STORAGE_SECRET_KEY")
	bindEnv(v, "redis.addr", "BBS_GATEWAY_REDIS_ADDR")
	bindEnv(v, "redis.password", "BBS_GATEWAY_REDIS_PASSWORD")
	bindEnv(v, "redis.db", "BBS_GATEWAY_REDIS_DB")
	bindEnv(v, "chat.rateLimit.ticketInterval", "BBS_GATEWAY_CHAT_RATE_LIMIT_TICKET_INTERVAL")
	bindEnv(v, "chat.rateLimit.ticketRate", "BBS_GATEWAY_CHAT_RATE_LIMIT_TICKET_RATE")
	bindEnv(v, "chat.rateLimit.createRoomInterval", "BBS_GATEWAY_CHAT_RATE_LIMIT_CREATE_ROOM_INTERVAL")
	bindEnv(v, "chat.rateLimit.createRoomRate", "BBS_GATEWAY_CHAT_RATE_LIMIT_CREATE_ROOM_RATE")
	bindEnv(v, "chat.rateLimit.subscribeInterval", "BBS_GATEWAY_CHAT_RATE_LIMIT_SUBSCRIBE_INTERVAL")
	bindEnv(v, "chat.rateLimit.subscribeRate", "BBS_GATEWAY_CHAT_RATE_LIMIT_SUBSCRIBE_RATE")
	bindEnv(v, "chat.rateLimit.joinInterval", "BBS_GATEWAY_CHAT_RATE_LIMIT_JOIN_INTERVAL")
	bindEnv(v, "chat.rateLimit.joinRate", "BBS_GATEWAY_CHAT_RATE_LIMIT_JOIN_RATE")
	bindEnv(v, "chat.rateLimit.sendInterval", "BBS_GATEWAY_CHAT_RATE_LIMIT_SEND_INTERVAL")
	bindEnv(v, "chat.rateLimit.sendRate", "BBS_GATEWAY_CHAT_RATE_LIMIT_SEND_RATE")
	bindEnv(v, "chat.rateLimit.readInterval", "BBS_GATEWAY_CHAT_RATE_LIMIT_READ_INTERVAL")
	bindEnv(v, "chat.rateLimit.readRate", "BBS_GATEWAY_CHAT_RATE_LIMIT_READ_RATE")
	bindEnv(v, "chat.websocket.maxConnectionsPerUser", "BBS_GATEWAY_CHAT_WEBSOCKET_MAX_CONNECTIONS_PER_USER")
	bindEnv(v, "chat.websocket.maxConnectionsPerIP", "BBS_GATEWAY_CHAT_WEBSOCKET_MAX_CONNECTIONS_PER_IP")
}

func bindEnv(v *viper.Viper, key string, envs ...string) {
	_ = v.BindEnv(append([]string{key}, envs...)...)
}

func applyEnvOverrides(v *viper.Viper) {
	if value := strings.TrimSpace(os.Getenv("BBS_GATEWAY_HTTP_HOST")); value != "" {
		v.Set("http.host", value)
	}
	if value := strings.TrimSpace(os.Getenv("BBS_GATEWAY_CORS_ALLOWED_ORIGINS")); value != "" {
		v.Set("cors.allowedOrigins", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_GATEWAY_HTTP_TRUSTED_PROXIES")); value != "" {
		v.Set("http.trustedProxies", splitCommaSeparated(value))
	}
	if value := strings.TrimSpace(os.Getenv("BBS_GATEWAY_GRPC_CLIENT_ETCD_ADDR")); value != "" {
		v.Set("grpc.client.etcdAddr", splitCommaSeparated(value))
	}
}

func splitCommaSeparated(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func isProductionEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "prod", "production":
		return true
	default:
		return false
	}
}

func applyLocalPublicBaseURLPortOverride(v *viper.Viper, port int) {
	if v == nil || port <= 0 || !isLocalDevelopmentEnvironment(v.GetString("trace.env")) {
		return
	}
	if strings.TrimSpace(os.Getenv("BBS_GATEWAY_SERVICE_HTTP_PORT")) == "" {
		return
	}
	configuredBaseURL := strings.TrimRight(strings.TrimSpace(v.GetString("http.publicBaseURL")), "/")
	if configuredBaseURL != defaultLocalGatewayPublicBaseURL || port == 18080 {
		return
	}
	v.Set("http.publicBaseURL", fmt.Sprintf("http://127.0.0.1:%d", port))
}

func isLocalDevelopmentEnvironment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "local", "dev", "development":
		return true
	default:
		return false
	}
}
