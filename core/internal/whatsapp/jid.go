package whatsapp

import "go.mau.fi/whatsmeow/types"

// normalizePhoneDigits strips any non-digit characters (spaces, +, dashes,
// parens...) from a phone number. Shared by the whatsmeow JID builder below
// and the Cloud API client, which both need the same bare-digits form —
// whatsmeow as the User part of a JID, Cloud API as its "to" field.
func normalizePhoneDigits(phone string) string {
	var digits string
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits += string(r)
		}
	}
	return digits
}

// toJID builds a WhatsApp JID from a phone number, stripping any non-digit
// characters (spaces, +, dashes, parens...).
func toJID(phone string) types.JID {
	return types.NewJID(normalizePhoneDigits(phone), types.DefaultUserServer)
}
