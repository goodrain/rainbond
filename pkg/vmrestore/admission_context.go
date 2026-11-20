package vmrestore

import "context"

type admissionContextKey struct{}
type admissionContextValue struct{ token string }

// WithAdmissionContext is only for an actor that already successfully claimed
// this token in Store. It must never be populated from a UI body or ordinary header.
func WithAdmissionContext(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, admissionContextKey{}, admissionContextValue{token: token})
}
func ownsAdmissionContext(ctx context.Context, scope Scope, currentToken string) bool {
	value, ok := ctx.Value(admissionContextKey{}).(admissionContextValue)
	// Scope was validated by Store.Read for the state supplying currentToken.
	return ok && validScope(scope) && currentToken != "" && value.token == currentToken
}
