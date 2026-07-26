// data.go — TLD reputation, known-legitimate origins, port sets and typosquat targets.
// Purpose: Defines TLD reputation data and suspicious domain lists for security flagging.
// Why: Separates static flagging data from detector logic and distance calculations.
package netflag

// TLDReputation defines risk level for a TLD.
type TLDReputation struct {
	Severity string // "low", "medium", "high"
	Reason   string
}

var suspiciousTLDs = map[string]TLDReputation{
	".xyz":      {"medium", "TLD .xyz has elevated abuse rates"},
	".top":      {"medium", "TLD .top frequently used for malicious domains"},
	".loan":     {"high", "TLD .loan commonly associated with phishing"},
	".click":    {"medium", "TLD .click has elevated spam rates"},
	".stream":   {"medium", "TLD .stream frequently used for piracy sites"},
	".download": {"high", "TLD .download associated with malware distribution"},
	".review":   {"medium", "TLD .review has elevated fraud rates"},
	".country":  {"medium", "TLD .country frequently used for scams"},
}

var knownLegitimateOrigins = map[string]string{
	"https://pages.dev":       "Cloudflare Pages (legitimate)",
	"https://vercel.app":      "Vercel hosting (legitimate)",
	"https://netlify.app":     "Netlify hosting (legitimate)",
	"https://railway.app":     "Railway hosting (legitimate)",
	"https://fly.dev":         "Fly.io hosting (legitimate)",
	"https://workers.dev":     "Cloudflare Workers (legitimate)",
	"https://web.app":         "Firebase Hosting (legitimate)",
	"https://firebaseapp.com": "Firebase Hosting (legitimate)",
}

var standardWebPorts = map[string]struct{}{
	"80":  {},
	"443": {},
}

var localhostDevPorts = map[string]struct{}{
	"3000": {},
	"3001": {},
	"4200": {},
	"5000": {},
	"5173": {},
	"8000": {},
	"8080": {},
	"9000": {},
}

var typosquatTargetDomains = []string{
	"unpkg.com",
	"jsdelivr.net",
	"cdnjs.cloudflare.com",
	"cloudflare.com",
	"googleapis.com",
	"gstatic.com",
	"jquery.com",
	"bootstrap.com",
}
