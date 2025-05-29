package plenticore

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

type Client struct {
	sessionID string
	apiURL    string
}

func NewClientOrDie(server, password string) *Client {
	c := &Client{
		apiURL: fmt.Sprintf("https://%s/api/v1", server),
	}

	username := "user" // default username of plant owner
	type authStartRequest struct {
		Nonce    string `json:"nonce"`
		Username string `json:"username"`
	}
	type authStartResponse struct {
		TransactionId string `json:"transactionId"`
		Nonce         string `json:"nonce"`
		Salt          string `json:"salt"`
		Rounds        int    `json:"rounds"`
	}
	startRequest := authStartRequest{
		Nonce:    base64.StdEncoding.EncodeToString(randomBytesOrDie(32)),
		Username: username,
	}
	var startResponse authStartResponse
	err := c.DoRequest(c.apiURL+"/auth/start", "POST", startRequest, &startResponse)
	if err != nil {
		logrus.Fatalf("could not initiate authentication: %v", err)
	}

	salt, err := base64.StdEncoding.DecodeString(startResponse.Salt)
	if err != nil {
		logrus.Fatalf("could not decode salt: %v", err)
	}
	saltedPassword, err := pbkdf2.Key(sha256.New, password, []byte(salt), startResponse.Rounds, 32)
	if err != nil {
		logrus.Fatalf("could not derive salted password: %v", err)
	}

	clientKey := HMACSHA256(saltedPassword, "Client Key")
	serverKey := HMACSHA256(saltedPassword, "Server Key")
	storedKey := SHA256Hash(clientKey)

	authMessage := fmt.Sprintf("n=%s,r=%s,r=%s,s=%s,i=%d,c=biws,r=%s",
		username,
		startRequest.Nonce,
		startResponse.Nonce,
		startResponse.Salt,
		startResponse.Rounds,
		startResponse.Nonce,
	)

	clientSignature := HMACSHA256(storedKey, authMessage)
	serverSignature := HMACSHA256(serverKey, authMessage)
	clientProof := CreateClientProof(clientSignature, clientKey)

	type authFinishRequest struct {
		TransactionId string `json:"transactionId"`
		Proof         string `json:"proof"`
	}
	type authFinishResponse struct {
		Signature string `json:"signature"`
		Token     string `json:"token"`
	}
	finishRequest := authFinishRequest{
		TransactionId: startResponse.TransactionId,
		Proof:         clientProof,
	}

	var finishResponse authFinishResponse
	err = c.DoRequest(c.apiURL+"/auth/finish", "POST", finishRequest, &finishResponse)
	if err != nil {
		logrus.Fatalf("could not finish authentication: %v", err)
	}

	signature, err := base64.StdEncoding.DecodeString(finishResponse.Signature)
	if err != nil {
		logrus.Fatalf("could not decode signature: %v", err)
	}
	if bytes.Compare(signature, serverSignature) != 0 {
		logrus.Fatalf("signature mismatch: expected %x, got %x", serverSignature, signature)
	}

	h := hmac.New(sha256.New, []byte(storedKey))
	h.Write([]byte("Session Key"))
	h.Write([]byte(authMessage))
	h.Write([]byte(clientKey))
	protocolKey := h.Sum(nil)
	ivNonce := randomBytesOrDie(16)
	block, err := aes.NewCipher(protocolKey)
	if err != nil {
		logrus.Fatalf("could not create AES cipher: %v", err)
	}
	aesgcm, err := cipher.NewGCMWithNonceSize(block, 16)
	if err != nil {
		logrus.Fatalf("could not create GCM: %v", err)
	}

	var tag []byte
	ciphertext := aesgcm.Seal(nil, ivNonce, []byte(finishResponse.Token), nil)
	// golang appends tag at the end of ciphertext, so we have to extract it
	// see https://stackoverflow.com/questions/68350301/extract-tag-from-cipher-aes-256-gcm-golang
	ciphertext, tag = ciphertext[:len(ciphertext)-16], ciphertext[len(ciphertext)-16:]

	// perform step 3 of authentication request to get session id
	type authCreateSessionRequest struct {
		TransactionId string `json:"transactionId"`
		Iv            string `json:"iv"`
		Tag           string `json:"tag"`
		Payload       string `json:"payload"`
	}
	type authCreateSessionResponse struct {
		SessionID string `json:"sessionId"`
	}
	createSessionRequest := authCreateSessionRequest{
		TransactionId: startResponse.TransactionId,
		Iv:            base64.StdEncoding.EncodeToString(ivNonce),
		Tag:           base64.StdEncoding.EncodeToString(tag),
		Payload:       base64.StdEncoding.EncodeToString(ciphertext),
	}
	var createSessionResponse authCreateSessionResponse
	err = c.DoRequest(c.apiURL+"/auth/create_session", "POST", createSessionRequest, &createSessionResponse)
	if err != nil {
		logrus.Fatalf("could not create session: %v", err)
	}
	c.sessionID = createSessionResponse.SessionID
	logrus.Info("Logged in successfully")

	return c
}

func (c *Client) Close() {
	err := c.DoRequest(c.apiURL+"/auth/logout", "POST", nil, nil)
	if err != nil {
		logrus.Errorf("Unable to log out: %v", err)
	} else {
		logrus.Info("Logged out successfully")
	}
	c.sessionID = ""
}

type Fields struct {
	ModuleID string   `json:"moduleid"`
	FieldIDs []string `json:"processdataids"`
}

func (c *Client) Fields() []Fields {
	res := make([]Fields, 0)
	err := c.DoRequest(c.apiURL+"/processdata", "GET", nil, &res)
	if err != nil {
		logrus.Errorf("Failed to process data: %v", err)
	}
	return res
}

func (c *Client) Data(fields []Fields) {
	err := c.DoRequest(c.apiURL+"/processdata", "POST", fields, nil)
	if err != nil {
		logrus.Errorf("Failed to retrieve data: %v", err)
		return
	}
}

func (c *Client) DoRequest(url string, method string, in any, out any) error {
	reqBody, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("could not marshal request body: %v", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.sessionID != "" {
		req.Header.Set("Authorization", "Session "+c.sessionID)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request returned with http error %s", resp.Status)
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("could not decode response body: %v", err)
		}
	}

	return nil
}

// LoggingRoundTripper is a custom RoundTripper that logs requests and responses
type LoggingRoundTripper struct {
	Next http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface
func (l LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Log the request
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		// Restore the request body
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
	}
	logrus.Debugf("->: %s %s\nBody: %s", req.Method, req.URL.String(), string(reqBody))

	// Perform the request
	resp, err := l.Next.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// Log the response
	if resp.Body != nil {
		respBody, _ := io.ReadAll(resp.Body)
		// Restore the response body
		resp.Body = io.NopCloser(bytes.NewBuffer(respBody))
		logrus.Debugf("<-: %s %s\nStatus: %d\nBody: %s", req.Method, req.URL.String(), resp.StatusCode, string(respBody))
	}

	return resp, nil
}

var httpClient = &http.Client{
	Transport: &LoggingRoundTripper{
		Next: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	},
}

func randomBytesOrDie(n int) []byte {
	bytes := make([]byte, n)
	_, err := rand.Read(bytes)
	if err != nil {
		logrus.Fatalf("could not generate random bytes: %v", err)
	}
	return bytes
}

func HMACSHA256(secret []byte, val string) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(val))
	return h.Sum(nil)
}

func SHA256Hash(val []byte) []byte {
	hash := sha256.Sum256(val)
	return hash[:]
}

// CreateClientProof returns the client proof computed by client and server signature as base64 encoded string
func CreateClientProof(clientSignature []byte, serverSignature []byte) string {
	n := len(clientSignature)
	res := make([]byte, n)
	for i := 0; i < len(clientSignature); i++ {
		res[i] = (byte(0xff & (clientSignature[i] ^ serverSignature[i])))
	}
	return base64.StdEncoding.EncodeToString(res)
}
