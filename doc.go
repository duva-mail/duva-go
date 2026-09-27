// Package duva is the official Duva client library. Duva is a transactional email API hosted in
// Canada. See https://duva.ca and https://duva.ca/en/docs for the full HTTP reference this
// package wraps.
//
//	client, err := duva.New("dv_...", "example.com") // or DUVA_API_KEY / DUVA_DOMAIN
//	message, err := client.Messages.Send(ctx, duva.SendMessageParams{
//		From:    "Example <notifications@example.com>",
//		To:      []string{"client@example.org"},
//		Subject: "Your order",
//		Text:    "Thank you for your order.",
//	})
package duva
