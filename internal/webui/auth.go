package webui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// WebAuth handles authentication for the Web UI.
type WebAuth struct {
	mu       sync.RWMutex
	users    map[string]*WebUser    // username -> user
	sessions map[string]*WebSession // token -> session
}

type WebUser struct {
	Username string `json:"username"`
	PassHash string `json:"pass_hash"`
	Salt     string `json:"salt"`
	Role     string `json:"role"` // admin, operator, viewer
}

type WebSession struct {
	Token     string
	Username  string
	Role      string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func NewWebAuth() *WebAuth {
	wa := &WebAuth{
		users:    make(map[string]*WebUser),
		sessions: make(map[string]*WebSession),
	}
	// Default admin user — operator should change this
	wa.CreateUser("admin", "phantom", "admin")
	return wa
}

func (wa *WebAuth) CreateUser(username, password, role string) {
	salt := randomHex(16)
	hash := hashPass(password, salt)
	wa.mu.Lock()
	wa.users[username] = &WebUser{Username: username, PassHash: hash, Salt: salt, Role: role}
	wa.mu.Unlock()
}

func (wa *WebAuth) Authenticate(username, password string) (string, error) {
	wa.mu.RLock()
	user, ok := wa.users[username]
	wa.mu.RUnlock()

	if !ok || bcrypt.CompareHashAndPassword([]byte(user.PassHash), []byte(password+user.Salt)) != nil {
		return "", fmt.Errorf("invalid credentials")
	}

	token := randomHex(32)
	wa.mu.Lock()
	wa.sessions[token] = &WebSession{
		Token: token, Username: username, Role: user.Role,
		CreatedAt: time.Now(), ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	wa.mu.Unlock()

	return token, nil
}

func (wa *WebAuth) ValidateRequest(r *http.Request) *WebSession {
	// Check cookie
	c, err := r.Cookie("phantom_session")
	if err != nil || c.Value == "" {
		return nil
	}

	wa.mu.RLock()
	session, ok := wa.sessions[c.Value]
	wa.mu.RUnlock()

	if !ok || time.Now().After(session.ExpiresAt) {
		return nil
	}
	return session
}

// AuthMiddleware wraps handlers requiring authentication.
// Supports both cookie-based sessions and API key authentication.
func (wa *WebAuth) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check session cookie first
		session := wa.ValidateRequest(r)
		if session != nil {
			next(w, r)
			return
		}
		// Check API key (for scripting/automation)
		if ValidateAPIKey(r) {
			next(w, r)
			return
		}
		// Login pages don't need auth
		if r.URL.Path == "/login" || r.URL.Path == "/api/login" {
			next(w, r)
			return
		}
		// API endpoints: return JSON 401 so fetch() gets parseable JSON, not HTML
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"session expired — please log in again"}`)
			return
		}
		http.Redirect(w, r, "/login", 302)
	}
}

func (wa *WebAuth) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, loginPageHTML)
		return
	}

	// POST — API login
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		json.NewDecoder(r.Body).Decode(&req)
	} else {
		req.Username = r.FormValue("username")
		req.Password = r.FormValue("password")
	}

	token, err := wa.Authenticate(req.Username, req.Password)
	if err != nil {
		if contentType == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
		} else {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, loginPageHTMLError)
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name: "phantom_session", Value: token, Path: "/",
		MaxAge: 86400, HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: r.TLS != nil,
	})

	if contentType == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "token": token})
	} else {
		http.Redirect(w, r, "/", 302)
	}
}

func (wa *WebAuth) HandleLogout(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Cookie("phantom_session")
	if c != nil {
		wa.mu.Lock()
		delete(wa.sessions, c.Value)
		wa.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "phantom_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil})
	http.Redirect(w, r, "/login", 302)
}

func (wa *WebAuth) GetOnlineOperators() []string {
	wa.mu.RLock()
	defer wa.mu.RUnlock()
	seen := map[string]bool{}
	var ops []string
	for _, s := range wa.sessions {
		if !time.Now().After(s.ExpiresAt) && !seen[s.Username] {
			ops = append(ops, s.Username)
			seen[s.Username] = true
		}
	}
	return ops
}

func hashPass(password, salt string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password+salt), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(hash)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

const loginPageHTML = `<!DOCTYPE html><html><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Phantom C2 — Login</title>
<style>*{margin:0;padding:0;box-sizing:border-box}
body{min-height:100vh;display:flex;align-items:center;justify-content:center;background:#05070f;font-family:'Segoe UI',system-ui,-apple-system,sans-serif;color:#eef1f8;overflow:hidden;padding:20px}
.bg{position:fixed;inset:0;z-index:-1;background:radial-gradient(900px 600px at 85% -10%,rgba(139,92,246,0.22),transparent 60%),radial-gradient(700px 500px at -10% 30%,rgba(34,211,238,0.14),transparent 55%),radial-gradient(600px 500px at 50% 110%,rgba(232,121,249,0.12),transparent 55%),#05070f;animation:drift 18s ease-in-out infinite}
@keyframes drift{0%,100%{transform:scale(1)}50%{transform:scale(1.06)}}
.card{display:flex;width:100%;max-width:780px;border-radius:24px;overflow:hidden;box-shadow:0 24px 80px rgba(0,0,0,0.6),0 0 60px rgba(139,92,246,0.15);border:1px solid #1c2440;background:rgba(14,20,36,0.92)}
.brand{flex:1.1;padding:48px 40px;background:linear-gradient(160deg,rgba(139,92,246,0.20),rgba(14,20,36,0.55));display:flex;flex-direction:column;justify-content:center;gap:16px;border-right:1px solid rgba(255,255,255,0.04)}
.brand .mark{width:58px;height:58px;border-radius:16px;display:flex;align-items:center;justify-content:center;font-size:28px;background:linear-gradient(135deg,rgba(139,92,246,0.3),rgba(232,121,249,0.2));border:1px solid rgba(139,92,246,0.35);box-shadow:0 0 30px rgba(139,92,246,0.25)}
.brand h1{font-size:28px;font-weight:800;letter-spacing:-0.5px;background:linear-gradient(135deg,#c4b5fd,#e879f9 55%,#a5f3fc);-webkit-background-clip:text;background-clip:text;-webkit-text-fill-color:transparent}
.brand p{color:#8b96b5;font-size:13px;line-height:1.7;max-width:280px}
.form{flex:1;padding:48px 40px}
.form h2{font-size:22px;font-weight:700;margin-bottom:4px}
.form .sub{color:#556183;font-size:12px;margin-bottom:26px}
.field{margin-bottom:16px}
.field label{display:block;font-size:11px;color:#8b96b5;text-transform:uppercase;letter-spacing:1px;margin-bottom:6px}
.field input{width:100%;padding:12px 14px;background:#0a101f;border:1px solid #273154;border-radius:10px;color:#eef1f8;font-size:14px;outline:none;transition:border .2s,box-shadow .2s}
.field input:focus{border-color:#8b5cf6;box-shadow:0 0 0 3px rgba(139,92,246,0.18)}
.btn{width:100%;padding:12px;background:linear-gradient(135deg,#8b5cf6,#e879f9 55%,#22d3ee);color:#fff;border:none;border-radius:10px;font-size:14px;font-weight:700;cursor:pointer;box-shadow:0 4px 20px rgba(139,92,246,0.35);transition:filter .2s,transform .2s}
.btn:hover{filter:brightness(1.12);transform:translateY(-1px)}
.error{background:rgba(244,63,94,0.12);color:#f43f5e;border:1px solid rgba(244,63,94,0.3);padding:11px;border-radius:10px;margin-bottom:16px;font-size:13px;text-align:center}
.foot{text-align:center;margin-top:22px;font-size:11px;color:#556183}
@media(max-width:640px){.card{flex-direction:column}.brand{padding:30px;border-right:none;border-bottom:1px solid rgba(255,255,255,0.04)}.form{padding:30px}}</style></head><body>
<div class="bg"></div>
<div class="card">
<div class="brand">
<div class="mark">🛡️</div>
<h1>Phantom C2</h1>
<p>Command &amp; Control for authorized red team operations. Stealth. Precision. Control.</p>
</div>
<div class="form">
<h2>Sign in</h2>
<div class="sub">Access your operations dashboard</div>
<form method="POST" action="/login">
<div class="field"><label>Username</label><input type="text" name="username" autofocus required></div>
<div class="field"><label>Password</label><input type="password" name="password" required></div>
<button type="submit" class="btn">Sign In</button>
</form>
<div class="foot">Phantom C2 Framework</div>
</div>
</div>
</body></html>`

const loginPageHTMLError = `<!DOCTYPE html><html><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Phantom C2 — Login</title>
<style>*{margin:0;padding:0;box-sizing:border-box}
body{min-height:100vh;display:flex;align-items:center;justify-content:center;background:#05070f;font-family:'Segoe UI',system-ui,-apple-system,sans-serif;color:#eef1f8;overflow:hidden;padding:20px}
.bg{position:fixed;inset:0;z-index:-1;background:radial-gradient(900px 600px at 85% -10%,rgba(139,92,246,0.22),transparent 60%),radial-gradient(700px 500px at -10% 30%,rgba(34,211,238,0.14),transparent 55%),radial-gradient(600px 500px at 50% 110%,rgba(232,121,249,0.12),transparent 55%),#05070f;animation:drift 18s ease-in-out infinite}
@keyframes drift{0%,100%{transform:scale(1)}50%{transform:scale(1.06)}}
.card{display:flex;width:100%;max-width:780px;border-radius:24px;overflow:hidden;box-shadow:0 24px 80px rgba(0,0,0,0.6),0 0 60px rgba(139,92,246,0.15);border:1px solid #1c2440;background:rgba(14,20,36,0.92)}
.brand{flex:1.1;padding:48px 40px;background:linear-gradient(160deg,rgba(139,92,246,0.20),rgba(14,20,36,0.55));display:flex;flex-direction:column;justify-content:center;gap:16px;border-right:1px solid rgba(255,255,255,0.04)}
.brand .mark{width:58px;height:58px;border-radius:16px;display:flex;align-items:center;justify-content:center;font-size:28px;background:linear-gradient(135deg,rgba(139,92,246,0.3),rgba(232,121,249,0.2));border:1px solid rgba(139,92,246,0.35);box-shadow:0 0 30px rgba(139,92,246,0.25)}
.brand h1{font-size:28px;font-weight:800;letter-spacing:-0.5px;background:linear-gradient(135deg,#c4b5fd,#e879f9 55%,#a5f3fc);-webkit-background-clip:text;background-clip:text;-webkit-text-fill-color:transparent}
.brand p{color:#8b96b5;font-size:13px;line-height:1.7;max-width:280px}
.form{flex:1;padding:48px 40px}
.form h2{font-size:22px;font-weight:700;margin-bottom:4px}
.form .sub{color:#556183;font-size:12px;margin-bottom:26px}
.field{margin-bottom:16px}
.field label{display:block;font-size:11px;color:#8b96b5;text-transform:uppercase;letter-spacing:1px;margin-bottom:6px}
.field input{width:100%;padding:12px 14px;background:#0a101f;border:1px solid #273154;border-radius:10px;color:#eef1f8;font-size:14px;outline:none;transition:border .2s,box-shadow .2s}
.field input:focus{border-color:#8b5cf6;box-shadow:0 0 0 3px rgba(139,92,246,0.18)}
.btn{width:100%;padding:12px;background:linear-gradient(135deg,#8b5cf6,#e879f9 55%,#22d3ee);color:#fff;border:none;border-radius:10px;font-size:14px;font-weight:700;cursor:pointer;box-shadow:0 4px 20px rgba(139,92,246,0.35);transition:filter .2s,transform .2s}
.btn:hover{filter:brightness(1.12);transform:translateY(-1px)}
.error{background:rgba(244,63,94,0.12);color:#f43f5e;border:1px solid rgba(244,63,94,0.3);padding:11px;border-radius:10px;margin-bottom:16px;font-size:13px;text-align:center}
.foot{text-align:center;margin-top:22px;font-size:11px;color:#556183}
@media(max-width:640px){.card{flex-direction:column}.brand{padding:30px;border-right:none;border-bottom:1px solid rgba(255,255,255,0.04)}.form{padding:30px}}</style></head><body>
<div class="bg"></div>
<div class="card">
<div class="brand">
<div class="mark">🛡️</div>
<h1>Phantom C2</h1>
<p>Command &amp; Control for authorized red team operations. Stealth. Precision. Control.</p>
</div>
<div class="form">
<h2>Sign in</h2>
<div class="sub">Access your operations dashboard</div>
<div class="error">Invalid username or password</div><form method="POST" action="/login">
<div class="field"><label>Username</label><input type="text" name="username" autofocus required></div>
<div class="field"><label>Password</label><input type="password" name="password" required></div>
<button type="submit" class="btn">Sign In</button>
</form>
<div class="foot">Phantom C2 Framework</div>
</div>
</div>
</body></html>`
