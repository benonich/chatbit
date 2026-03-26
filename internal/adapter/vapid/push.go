package vapid

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/SherClockHolmes/webpush-go"
)

type VapidKeys struct {
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
}

var (
	keys         VapidKeys
	subMu        sync.Mutex
	subscription *webpush.Subscription // Demo: eine Subscription im Speicher
)

// GenerateVAPIDKeys will create a private and public VAPID key pair
func GenerateVAPIDKeys() (privateKey, publicKey string, err error) {
	// Get the private key from the P256 curve
	priv, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return
	}

	// PublicKey returns the uncompressed byte format (65 bytes) by default for P256
	pub := priv.PublicKey().Bytes()
	privBytes := priv.Bytes()

	// Convert to URL-safe base64 without padding as per VAPID spec
	publicKey = base64.RawURLEncoding.EncodeToString(pub)
	privateKey = base64.RawURLEncoding.EncodeToString(privBytes)

	return
}

func ensureVapidKeys(path string) (VapidKeys, error) {
	// falls vorhanden, laden
	if b, err := os.ReadFile(path); err == nil {
		var k VapidKeys
		if err := json.Unmarshal(b, &k); err != nil {
			return VapidKeys{}, err
		}
		if k.PublicKey != "" && k.PrivateKey != "" {
			return k, nil
		}
	}

	// sonst erzeugen
	priv, pub, err := GenerateVAPIDKeys()
	if err != nil {
		return VapidKeys{}, err
	}
	k := VapidKeys{PublicKey: pub, PrivateKey: priv}

	b, _ := json.MarshalIndent(k, "", "  ")
	if err := os.WriteFile(path, b, 0600); err != nil {
		return VapidKeys{}, err
	}
	log.Println("✅ VAPID Keys erzeugt:", path)
	log.Println(pub)
	return k, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func GenerateVapID() {
	// VAPID Keys (einmalig) in server/vapid.json speichern
	var err error
	keys, err = ensureVapidKeys("vapid.json")
	if err != nil {
		log.Fatal(err)
	}
}

// --- API ---
var VapIDPubKeyFunc = func(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"publicKey": keys.PublicKey})
}

var SubscribeFunc = func(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	var sub webpush.Subscription
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid subscription json"})
		return
	}
	subMu.Lock()
	subscription = &sub
	subMu.Unlock()

	writeJSON(w, 201, map[string]bool{"ok": true})
}

var SendFunc = func(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}

	subMu.Lock()
	sub := subscription
	subMu.Unlock()
	if sub == nil {
		writeJSON(w, 400, map[string]string{"error": "Keine Subscription vorhanden. Erst /subscribe."})
		return
	}

	// Payload wie im SW erwartet
	var req struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		URL   string `json:"url"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Title == "" {
		req.Title = "Hallo von der PWA 👋"
	}
	if req.Body == "" {
		req.Body = "Das ist eine Test Push Notification."
	}
	if req.URL == "" {
		req.URL = "/"
	}

	payload, _ := json.Marshal(map[string]any{
		"title": req.Title,
		"body":  req.Body,
		"url":   req.URL,
	})

	resp, err := webpush.SendNotification(payload, sub, &webpush.Options{
		Subscriber:      "alessandro@chatbit.ch",
		VAPIDPublicKey:  keys.PublicKey,
		VAPIDPrivateKey: keys.PrivateKey,
		TTL:             60, // Sekunden
	})
	if err != nil {
		log.Println("❌ Push Fehler:", err)
		writeJSON(w, 500, map[string]string{"error": "Push fehlgeschlagen", "details": err.Error()})
		return
	}
	_ = resp.Body.Close()

	writeJSON(w, 200, map[string]bool{"ok": true})
}
