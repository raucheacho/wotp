// Package project provides the multi-tenant registry that lazily loads and
// caches a per-project runtime (store, OTP engine, templates, webhooks) on a
// wotp-core instance. See core/internal/store/control.go for the shared
// control-plane data (projects, api_keys, numbers) this package builds on.
package project

// Settings holds the per-project configuration knobs that used to live in
// the single global config.Config (OTPConfig/MessagingConfig/WebhooksConfig/
// TemplatesConfig.DefaultLocale, plus the WhatsAppConfig inbound filters).
// Stored as a JSON blob on store.Project.SettingsJSON — these fields are
// read-mostly and never filtered on in SQL, so a wide dedicated table isn't
// worth the migration churn every time a knob is added.
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
	// project's OTP sends (see whatsapp.CloudClient) — per-project, not
	// instance-wide, so two projects on the same wotp-core instance never
	// share a Cloud API number/token. Set via PATCH /v1/projects/{id}/settings
	// (or the dashboard's project settings screen). When Enabled is false
	// (the default), the project sends OTPs through whatsmeow as before.
	Cloud struct {
		Enabled             bool   `json:"enabled"`
		PhoneNumberID       string `json:"phone_number_id"`
		AccessToken         string `json:"access_token"`
		OTPTemplateName     string `json:"otp_template_name"`
		OTPTemplateLanguage string `json:"otp_template_language"`
	} `json:"cloud"`
}

// DefaultSettings returns the settings applied to a newly created project,
// matching core/internal/config.Defaults() so behavior doesn't change for a
// freshly created project versus today's single global config.
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
