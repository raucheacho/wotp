// Package project builds the single Runtime (store, OTP engine, templates,
// webhooks, WhatsApp pool) that backs a wotp-core instance. wotp-core is
// mono-tenant: one instance, one WhatsApp number, one Runtime — see
// core/internal/store/control.go for the shared control-plane data
// (api_keys, numbers, settings) it's built from.
package project

// Settings holds the instance's configuration knobs — OTP, messaging,
// WhatsApp inbound filters, webhooks, templates, and the optional Cloud API
// backend. Stored as a JSON blob in control.db's instance_settings row (see
// store.ControlStore.GetSettings/UpdateSettings) rather than a wide
// dedicated table, since these fields are read-mostly and never filtered on
// in SQL.
type Settings struct {
	OTP struct {
		CodeLength             int `json:"code_length"`
		ExpiryMinutes          int `json:"expiry_minutes"`
		MaxAttempts            int `json:"max_attempts"`
		RateLimitPerPhonePerHr int `json:"rate_limit_per_phone_per_hour"`
	} `json:"otp"`
	Messaging struct {
		MaxMessagesPerMinute int  `json:"max_messages_per_minute"`
		SimulateTyping       bool `json:"simulate_typing"`
	} `json:"messaging"`
	WhatsApp struct {
		// IgnoreGroups/IgnoreStatus: safe defaults are true — see
		// core/internal/config/config.go WhatsAppConfig for the rationale.
		IgnoreGroups bool `json:"ignore_groups"`
		IgnoreStatus bool `json:"ignore_status"`
	} `json:"whatsapp"`
	Webhooks struct {
		Endpoint string   `json:"endpoint"`
		Events   []string `json:"events"`
		Secret   string   `json:"secret"`
	} `json:"webhooks"`
	Templates struct {
		DefaultLocale string `json:"default_locale"`
	} `json:"templates"`
	// Cloud configures the Meta WhatsApp Cloud API backend for this
	// instance's sends (see whatsapp.CloudClient). Set via PATCH
	// /v1/settings or the dashboard's Numbers screen. When Enabled is
	// false (the default), sends go through whatsmeow instead.
	Cloud struct {
		Enabled             bool   `json:"enabled"`
		PhoneNumberID       string `json:"phone_number_id"`
		AccessToken         string `json:"access_token"`
		OTPTemplateName     string `json:"otp_template_name"`
		OTPTemplateLanguage string `json:"otp_template_language"`
		// WabaID/Pin are only needed to register this number for inbound
		// webhook delivery (see whatsapp.CloudClient.RegisterPhoneNumber/
		// SubscribeWabaToApp) — left empty, registration is skipped and
		// this stays a send-only (OTP/messages) Cloud setup.
		WabaID string `json:"waba_id,omitempty"`
		Pin    string `json:"pin,omitempty"`
		// AppSecret/VerifyToken authenticate the inbound webhook receiver
		// (POST/GET /webhooks/meta) — required for inbound to work at all,
		// irrelevant otherwise.
		AppSecret   string `json:"app_secret,omitempty"`
		VerifyToken string `json:"verify_token,omitempty"`
	} `json:"cloud"`
}

// DefaultSettings returns the settings applied to a freshly initialized
// instance, matching core/internal/config.Defaults() so behavior doesn't
// change versus today's single global config.
func DefaultSettings() Settings {
	var s Settings
	s.OTP.CodeLength = 6
	s.OTP.ExpiryMinutes = 5
	s.OTP.MaxAttempts = 5
	s.OTP.RateLimitPerPhonePerHr = 3
	s.Messaging.MaxMessagesPerMinute = 60
	s.Messaging.SimulateTyping = false
	s.WhatsApp.IgnoreGroups = true
	s.WhatsApp.IgnoreStatus = true
	s.Webhooks.Events = []string{}
	s.Templates.DefaultLocale = "en"
	return s
}
