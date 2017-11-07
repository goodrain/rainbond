/*

This file implements functionality very similar to that of the xauth module in czmq.

Notable differences in here:

 - domains are supported
 - domains are used in AuthAllow and AuthDeny too
 - usernames/passwords are read from memory, not from file
 - public keys are read from memory, not from file
 - an address can be a single IP address, or an IP address and mask in CIDR notation
 - additional functions for configuring server or client socket with a single command

*/

package zmq4

/*
#include <zmq.h>
#include <stdlib.h>

#if ZMQ_VERSION_MINOR < 2
// Version < 4.2.x

int zmq_curve_public (char *z85_public_key, const char *z85_secret_key) { return 0; }

#endif // Version < 4.2.x
*/
import "C"

import (
	"errors"
	"log"
	"net"
	"strings"
	"unsafe"
)

const CURVE_ALLOW_ANY = "*"

var (
	auth_handler *Socket
	auth_quit    *Socket

	auth_init    = false
	auth_verbose = false

	auth_allow     = make(map[string]map[string]bool)
	auth_deny      = make(map[string]map[string]bool)
	auth_allow_net = make(map[string][]*net.IPNet)
	auth_deny_net  = make(map[string][]*net.IPNet)

	auth_users = make(map[string]map[string]string)

	auth_pubkeys = make(map[string]map[string]bool)

	auth_meta_handler = auth_meta_handler_default
)

func auth_meta_handler_default(version, request_id, domain, address, identity, mechanism string, credentials ...string) (metadata map[string]string) {
	return map[string]string{}
}

func auth_isIP(addr string) bool {
	if net.ParseIP(addr) != nil {
		return true
	}
	if _, _, err := net.ParseCIDR(addr); err == nil {
		return true
	}
	return false
}

func auth_is_allowed(domain, address string) bool {
	for _, d := range []string{domain, "*"} {
		if a, ok := auth_allow[d]; ok {
			if a[address] {
				return true
			}
		}
	}
	addr := net.ParseIP(address)
	if addr != nil {
		for _, d := range []string{domain, "*"} {
			if a, ok := auth_allow_net[d]; ok {
				for _, m := range a {
					if m.Contains(addr) {
						return true
					}
				}
			}
		}
	}
	return false
}

func auth_is_denied(domain, address string) bool {
	for _, d := range []string{domain, "*"} {
		if a, ok := auth_deny[d]; ok {
			if a[address] {
				return true
			}
		}
	}
	addr := net.ParseIP(address)
	if addr != nil {
		for _, d := range []string{domain, "*"} {
			if a, ok := auth_deny_net[d]; ok {
				for _, m := range a {
					if m.Contains(addr) {
						return true
					}
				}
			}
		}
	}
	return false
}

func auth_has_allow(domain string) bool {
	for _, d := range []string{domain, "*"} {
		if a, ok := auth_allow[d]; ok {
			if len(a) > 0 || len(auth_allow_net[d]) > 0 {
				return true
			}
		}
	}
	return false
}

func auth_has_deny(domain string) bool {
	for _, d := range []string{domain, "*"} {
		if a, ok := auth_deny[d]; ok {
			if len(a) > 0 || len(auth_deny_net[d]) > 0 {
				return true
			}
		}
	}
	return false
}

func auth_do_handler() {
	for {

		msg, err := auth_handler.RecvMessage(0)
		if err != nil {
			if auth_verbose {
				log.Println("AUTH: Quitting:", err)
			}
			break
		}

		if msg[0] == "QUIT" {
			if auth_verbose {
				log.Println("AUTH: Quitting: received QUIT message")
			}
			_, err := auth_handler.SendMessage("QUIT")
			if err != nil && auth_verbose {
				log.Println("AUTH: Quitting: bouncing QUIT message:", err)
			}
			break
		}

		version := msg[0]
		if version != "1.0" {
			panic("AUTH: version != 1.0")
		}

		request_id := msg[1]
		domain := msg[2]
		address := msg[3]
		identity := msg[4]
		mechanism := msg[5]
		credentials := msg[6:]

		username := ""
		password := ""
		client_key := ""
		if mechanism == "PLAIN" {
			username = msg[6]
			password = msg[7]
		} else if mechanism == "CURVE" {
			s := msg[6]
			if len(s) != 32 {
				panic("AUTH: len(client_key) != 32")
			}
			client_key = Z85encode(s)
		}

		allowed := false
		denied := false

		if auth_has_allow(domain) {
			if auth_is_allowed(domain, address) {
				allowed = true
				if auth_verbose {
					log.Printf("AUTH: PASSED (whitelist) domain=%q address=%q\n", domain, address)
				}
			} else {
				denied = true
				if auth_verbose {
					log.Printf("AUTH: DENIED (not in whitelist) domain=%q address=%q\n", domain, address)
				}
			}
		} else if auth_has_deny(domain) {
			if auth_is_denied(domain, address) {
				denied = true
				if auth_verbose {
					log.Printf("AUTH: DENIED (blacklist) domain=%q address=%q\n", domain, address)
				}
			} else {
				allowed = true
				if auth_verbose {
					log.Printf("AUTH: PASSED (not in blacklist) domain=%q address=%q\n", domain, address)
				}
			}
		}

		// Mechanism-specific checks
		if !denied {
			if mechanism == "NULL" && !allowed {
				// For NULL, we allow if the address wasn't blacklisted
				if auth_verbose {
					log.Printf("AUTH: ALLOWED (NULL)\n")
				}
				allowed = true
			} else if mechanism == "PLAIN" {
				// For PLAIN, even a whitelisted address must authenticate
				allowed = authenticate_plain(domain, username, password)
			} else if mechanism == "CURVE" {
				// For CURVE, even a whitelisted address must authenticate
				allowed = authenticate_curve(domain, client_key)
			}
		}
		if allowed {
			m := auth_meta_handler(version, request_id, domain, address, identity, mechanism, credentials...)
			user_id := ""
			if uid, ok := m["User-Id"]; ok {
				user_id = uid
				delete(m, "User-Id")
			}
			metadata := make([]byte, 0)
			for key, value := range m {
				if len(key) < 256 {
					metadata = append(metadata, auth_meta_blob(key, value)...)
				}
			}
			auth_handler.SendMessage(version, request_id, "200", "OK", user_id, metadata)
		} else {
			auth_handler.SendMessage(version, request_id, "400", "NO ACCESS", "", "")
		}
	}

	err := auth_handler.Close()
	if err != nil && auth_verbose {
		log.Println("AUTH: Quitting: Close:", err)
	}
	if auth_verbose {
		log.Println("AUTH: Quit")
	}
}

func authenticate_plain(domain, username, password string) bool {
	for _, dom := range []string{domain, "*"} {
		if m, ok := auth_users[dom]; ok {
			if m[username] == password {
				if auth_verbose {
					log.Printf("AUTH: ALLOWED (PLAIN) domain=%q username=%q password=%q\n", dom, username, password)
				}
				return true
			}
		}
	}
	if auth_verbose {
		log.Printf("AUTH: DENIED (PLAIN) domain=%q username=%q password=%q\n", domain, username, password)
	}
	return false
}

func authenticate_curve(domain, client_key string) bool {
	for _, dom := range []string{domain, "*"} {
		if m, ok := auth_pubkeys[dom]; ok {
			if m[CURVE_ALLOW_ANY] {
				if auth_verbose {
					log.Printf("AUTH: ALLOWED (CURVE any client) domain=%q\n", dom)
				}
				return true
			}
			if m[client_key] {
				if auth_verbose {
					log.Printf("AUTH: ALLOWED (CURVE) domain=%q client_key=%q\n", dom, client_key)
				}
				return true
			}
		}
	}
	if auth_verbose {
		log.Printf("AUTH: DENIED (CURVE) domain=%q client_key=%q\n", domain, client_key)
	}
	return false
}

// Start authentication.
//
// Note that until you add policies, all incoming NULL connections are allowed
// (classic ZeroMQ behaviour), and all PLAIN and CURVE connections are denied.
func AuthStart() (err error) {
	if auth_init {
		if auth_verbose {
			log.Println("AUTH: Already running")
		}
		return errors.New("Auth is already running")
	}

	auth_handler, err = NewSocket(REP)
	if err != nil {
		return
	}
	auth_handler.SetLinger(0)
	err = auth_handler.Bind("inproc://zeromq.zap.01")
	if err != nil {
		auth_handler.Close()
		return
	}

	auth_quit, err = NewSocket(REQ)
	if err != nil {
		auth_handler.Close()
		return
	}
	auth_quit.SetLinger(0)
	err = auth_quit.Connect("inproc://zeromq.zap.01")
	if err != nil {
		auth_handler.Close()
		auth_quit.Close()
		return
	}

	go auth_do_handler()

	if auth_verbose {
		log.Println("AUTH: Starting")
	}

	auth_init = true

	return
}

// Stop authentication.
func AuthStop() {
	if !auth_init {
		if auth_verbose {
			log.Println("AUTH: Not running, can't stop")
		}
		return
	}
	if auth_verbose {
		log.Println("AUTH: Stopping")
	}
	_, err := auth_quit.SendMessageDontwait("QUIT")
	if err != nil && auth_verbose {
		log.Println("AUTH: Stopping: SendMessageDontwait(\"QUIT\"):", err)
	}
	_, err = auth_quit.RecvMessage(0)
	if err != nil && auth_verbose {
		log.Println("AUTH: Stopping: RecvMessage:", err)
	}
	err = auth_quit.Close()
	if err != nil && auth_verbose {
		log.Println("AUTH: Stopping: Close:", err)
	}
	if auth_verbose {
		log.Println("AUTH: Stopped")
	}

	auth_init = false

}

// Allow (whitelist) some addresses for a domain.
//
// An address can be a single IP address, or an IP address and mask in CIDR notation.
//
// For NULL, all clients from these addresses will be accepted.
//
// For PLAIN and CURVE, they will be allowed to continue with authentication.
//
// You can call this method multiple times to whitelist multiple IP addresses.
//
// If you whitelist a single address for a domain, any non-whitelisted addresses
// for that domain are treated as blacklisted.
//
// Use domain "*" for all domains.
//
// For backward compatibility: if domain can be parsed as an IP address, it will be
// interpreted as another address, and it and all remaining addresses will be added
// to all domains.
func AuthAllow(domain string, addresses ...string) {
	if auth_isIP(domain) {
		auth_allow_for_domain("*", domain)
		auth_allow_for_domain("*", addresses...)
	} else {
		auth_allow_for_domain(domain, addresses...)
	}
}

func auth_allow_for_domain(domain string, addresses ...string) {
	if _, ok := auth_allow[domain]; !ok {
		auth_allow[domain] = make(map[string]bool)
		auth_allow_net[domain] = make([]*net.IPNet, 0)
	}
	for _, address := range addresses {
		if _, ipnet, err := net.ParseCIDR(address); err == nil {
			auth_allow_net[domain] = append(auth_allow_net[domain], ipnet)
		} else if net.ParseIP(address) != nil {
			auth_allow[domain][address] = true
		} else {
			if auth_verbose {
				log.Printf("AUTH: Allow for domain %q: %q is not a valid address or network\n", domain, address)
			}
		}
	}
}

// Deny (blacklist) some addresses for a domain.
//
// An address can be a single IP address, or an IP address and mask in CIDR notation.
//
// For all security mechanisms, this rejects the connection without any further authentication.
//
// Use either a whitelist for a domain, or a blacklist for a domain, not both.
// If you define both a whitelist and a blacklist for a domain, only the whitelist takes effect.
//
// Use domain "*" for all domains.
//
// For backward compatibility: if domain can be parsed as an IP address, it will be
// interpreted as another address, and it and all remaining addresses will be added
// to all domains.
func AuthDeny(domain string, addresses ...string) {
	if auth_isIP(domain) {
		auth_deny_for_domain("*", domain)
		auth_deny_for_domain("*", addresses...)
	} else {
		auth_deny_for_domain(domain, addresses...)
	}
}

func auth_deny_for_domain(domain string, addresses ...string) {
	if _, ok := auth_deny[domain]; !ok {
		auth_deny[domain] = make(map[string]bool)
		auth_deny_net[domain] = make([]*net.IPNet, 0)
	}
	for _, address := range addresses {
		if _, ipnet, err := net.ParseCIDR(address); err == nil {
			auth_deny_net[domain] = append(auth_deny_net[domain], ipnet)
		} else if net.ParseIP(address) != nil {
			auth_deny[domain][address] = true
		} else {
			if auth_verbose {
				log.Printf("AUTH: Deny for domain %q: %q is not a valid address or network\n", domain, address)
			}
		}
	}
}

// Add a user for PLAIN authentication for a given domain.
//
// Set `domain` to "*" to apply to all domains.
func AuthPlainAdd(domain, username, password string) {
	if _, ok := auth_users[domain]; !ok {
		auth_users[domain] = make(map[string]string)
	}
	auth_users[domain][username] = password
}

// Remove users from PLAIN authentication for a given domain.
func AuthPlainRemove(domain string, usernames ...string) {
	if u, ok := auth_users[domain]; ok {
		for _, username := range usernames {
			delete(u, username)
		}
	}
}

// Remove all users from PLAIN authentication for a given domain.
func AuthPlainRemoveAll(domain string) {
	delete(auth_users, domain)
}

// Add public user keys for CURVE authentication for a given domain.
//
// To cover all domains, use "*".
//
// Public keys are in Z85 printable text format.
//
// To allow all client keys without checking, specify CURVE_ALLOW_ANY for the key.
func AuthCurveAdd(domain string, pubkeys ...string) {
	if _, ok := auth_pubkeys[domain]; !ok {
		auth_pubkeys[domain] = make(map[string]bool)
	}
	for _, key := range pubkeys {
		auth_pubkeys[domain][key] = true
	}
}

// Remove user keys from CURVE authentication for a given domain.
func AuthCurveRemove(domain string, pubkeys ...string) {
	if p, ok := auth_pubkeys[domain]; ok {
		for _, pubkey := range pubkeys {
			delete(p, pubkey)
		}
	}
}

// Remove all user keys from CURVE authentication for a given domain.
func AuthCurveRemoveAll(domain string) {
	delete(auth_pubkeys, domain)
}

// Enable verbose tracing of commands and activity.
func AuthSetVerbose(verbose bool) {
	auth_verbose = verbose
}

/*
This function sets the metadata handler that is called by the ZAP
handler to retrieve key/value properties that should be set on reply
messages in case of a status code "200" (succes).

Default properties are `Socket-Type`, which is already set, and
`Identity` and `User-Id` that are empty by default. The last two can be
set, and more properties can be added.

The `User-Id` property is used for the `user id` frame of the reply
message. All other properties are stored in the `metadata` frame of the
reply message.

The default handler returns an empty map.

For the meaning of the handler arguments, and other details, see:
http://rfc.zeromq.org/spec:27#toc10
*/
func AuthSetMetadataHandler(
	handler func(
		version, request_id, domain, address, identity, mechanism string, credentials ...string) (metadata map[string]string)) {
	auth_meta_handler = handler
}

/*
This encodes a key/value pair into the format used by a ZAP handler.

Returns an error if key is more then 255 characters long.
*/
func AuthMetaBlob(key, value string) (blob []byte, err error) {
	if len(key) > 255 {
		return []byte{}, errors.New("Key too long")
	}
	return auth_meta_blob(key, value), nil
}

func auth_meta_blob(name, value string) []byte {
	l1 := len(name)
	l2 := len(value)
	b := make([]byte, l1+l2+5)
	b[0] = byte(l1)
	b[l1+1] = byte(l2 >> 24 & 255)
	b[l1+2] = byte(l2 >> 16 & 255)
	b[l1+3] = byte(l2 >> 8 & 255)
	b[l1+4] = byte(l2 & 255)
	copy(b[1:], []byte(name))
	copy(b[5+l1:], []byte(value))
	return b
}

//. Additional functions for configuring server or client socket with a single command

// Set NULL server role.
func (server *Socket) ServerAuthNull(domain string) error {
	err := server.SetPlainServer(0)
	if err == nil {
		err = server.SetZapDomain(domain)
	}
	return err
}

// Set PLAIN server role.
func (server *Socket) ServerAuthPlain(domain string) error {
	err := server.SetPlainServer(1)
	if err == nil {
		err = server.SetZapDomain(domain)
	}
	return err
}

// Set CURVE server role.
func (server *Socket) ServerAuthCurve(domain, secret_key string) error {
	err := server.SetCurveServer(1)
	if err == nil {
		err = server.SetCurveSecretkey(secret_key)
	}
	if err == nil {
		err = server.SetZapDomain(domain)
	}
	return err
}

// Set PLAIN client role.
func (client *Socket) ClientAuthPlain(username, password string) error {
	err := client.SetPlainUsername(username)
	if err == nil {
		err = client.SetPlainPassword(password)
	}
	return err
}

// Set CURVE client role.
func (client *Socket) ClientAuthCurve(server_public_key, client_public_key, client_secret_key string) error {
	err := client.SetCurveServerkey(server_public_key)
	if err == nil {
		err = client.SetCurvePublickey(client_public_key)
	}
	if err == nil {
		client.SetCurveSecretkey(client_secret_key)
	}
	return err
}

// Helper function to derive z85 public key from secret key
//
// Returns ErrorNotImplemented42 with ZeroMQ version < 4.2
func AuthCurvePublic(z85SecretKey string) (z85PublicKey string, err error) {
	if minor < 2 {
		return "", ErrorNotImplemented42
	}
	secret := C.CString(z85SecretKey)
	defer C.free(unsafe.Pointer(secret))
	public := C.CString(strings.Repeat(" ", 41))
	defer C.free(unsafe.Pointer(public))
	if i, err := C.zmq_curve_public(public, secret); int(i) != 0 {
		return "", errget(err)
	}
	z85PublicKey = C.GoString(public)
	return z85PublicKey, nil
}
