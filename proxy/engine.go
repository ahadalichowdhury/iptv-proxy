package proxy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"syscall"
	"time"

	"tv-proxy-go/utils"
)

const (
	defaultMaxConcurrent    = 1000
	defaultManifestTimeout  = 30 * time.Second
	defaultMaxManifestSize  = 1 << 20 // 1 MiB
	defaultMaxManifestLine  = 1 << 20 // 1 MiB per line
	defaultMaxRedirectHops  = 10
	masterPlaylistCacheTTL  = 2 * time.Minute
	vodPlaylistCacheTTL     = 5 * time.Minute
)

var errManifestLineTooLong = errors.New("manifest line exceeds buffer limit")

type upstreamKind int

const (
	upstreamManifest upstreamKind = iota
	upstreamStream
)

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

// Engine limits concurrent streams and proxies IPTV/HLS traffic to clients.
type Engine struct {
	sem              chan struct{}
	client           *http.Client
	cache            sync.Map
	maxConcurrent    int
	proxyBase        string
	tokenSecret      string
	manifestTimeout  time.Duration
	streamTimeout    time.Duration
	maxManifestSize  int64
	maxManifestLine  int
}

// NewEngine creates a proxy engine with the given concurrency limit.
func NewEngine(maxConcurrent int, proxyBase, tokenSecret string) *Engine {
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrent
	}

	return &Engine{
		sem: make(chan struct{}, maxConcurrent),
		client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				MaxIdleConns:          512,
				MaxIdleConnsPerHost:   128,
				MaxConnsPerHost:       0,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				ResponseHeaderTimeout: 15 * time.Second,
				DisableCompression:    true,
			},
		},
		maxConcurrent:   maxConcurrent,
		proxyBase:       strings.TrimRight(proxyBase, "/"),
		tokenSecret:     strings.TrimSpace(tokenSecret),
		manifestTimeout: defaultManifestTimeout,
		streamTimeout:   0, // 0 = no deadline beyond client disconnect for segment bodies
		maxManifestSize: defaultMaxManifestSize,
		maxManifestLine: defaultMaxManifestLine,
	}
}

// SetManifestTimeout configures the upstream deadline for manifest fetches.
func (e *Engine) SetManifestTimeout(d time.Duration) {
	if d > 0 {
		e.manifestTimeout = d
	}
}

// SetStreamTimeout configures an optional upstream deadline for segment streaming (0 = none).
func (e *Engine) SetStreamTimeout(d time.Duration) {
	e.streamTimeout = d
}

// SetMaxManifestSize configures the maximum allowed manifest payload size in bytes.
func (e *Engine) SetMaxManifestSize(size int64) {
	if size > 0 {
		e.maxManifestSize = size
	}
}

// Shutdown closes idle upstream connections. Call during process shutdown.
func (e *Engine) Shutdown() {
	e.client.CloseIdleConnections()
}

// HandleStream is the HTTP handler for /proxy?url=...&auth=...
func (e *Engine) HandleStream(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		e.handleStreamGet(w, r)
	case http.MethodPost:
		e.handleStreamPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (e *Engine) handleStreamGet(w http.ResponseWriter, r *http.Request) {
	targetURL, auth, err := e.resolveTarget(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, utils.ErrPlayTokenExpired) {
			status = http.StatusUnauthorized
		}
		http.Error(w, err.Error(), status)
		return
	}

	if _, err := url.ParseRequestURI(targetURL); err != nil {
		http.Error(w, "invalid url parameter", http.StatusBadRequest)
		return
	}

	if !isUpstreamHTTPURL(targetURL) {
		http.Error(w, "url must use http or https", http.StatusBadRequest)
		return
	}

	if err := e.acquire(r.Context()); err != nil {
		http.Error(w, "server at capacity, try again later", http.StatusServiceUnavailable)
		return
	}
	defer e.release()

	if e.isHLSManifestURL(targetURL) {
		e.serveHLSManifest(w, r, targetURL, auth)
		return
	}

	if isDashManifestURL(targetURL) {
		e.serveDashManifest(w, r, targetURL, auth)
		return
	}

	e.streamBinary(w, r, targetURL, auth)
}

func (e *Engine) handleStreamPost(w http.ResponseWriter, r *http.Request) {
	targetURL, auth, err := e.resolveTarget(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, utils.ErrPlayTokenExpired) {
			status = http.StatusUnauthorized
		}
		http.Error(w, err.Error(), status)
		return
	}

	if _, err := url.ParseRequestURI(targetURL); err != nil {
		http.Error(w, "invalid url parameter", http.StatusBadRequest)
		return
	}

	if !isUpstreamHTTPURL(targetURL) {
		http.Error(w, "url must use http or https", http.StatusBadRequest)
		return
	}

	if err := e.acquire(r.Context()); err != nil {
		http.Error(w, "server at capacity, try again later", http.StatusServiceUnavailable)
		return
	}
	defer e.release()

	e.proxyPost(w, r, targetURL, auth)
}

func (e *Engine) resolveTarget(r *http.Request) (targetURL, auth string, err error) {
	if token := strings.TrimSpace(r.URL.Query().Get("t")); token != "" {
		if e.tokenSecret == "" {
			return "", "", fmt.Errorf("opaque play tokens are not enabled")
		}
		return utils.DecryptPlayToken(e.tokenSecret, token)
	}

	targetURL = strings.TrimSpace(r.URL.Query().Get("url"))
	if targetURL == "" {
		return "", "", fmt.Errorf("missing required query parameter: url or t")
	}

	auth = r.URL.Query().Get("auth")
	if pipeURL, pipeAuth := utils.ParseStreamSource(targetURL); pipeAuth != "" || pipeURL != targetURL {
		targetURL = pipeURL
		auth = utils.MergeAuth(pipeAuth, auth)
	}

	return targetURL, auth, nil
}

func (e *Engine) proxyPost(w http.ResponseWriter, r *http.Request, targetURL, auth string) {
	upstreamAuth := utils.StripDrmAuth(auth)

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), e.manifestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusBadGateway)
		return
	}

	for k, values := range utils.HeadersForURL(targetURL, upstreamAuth) {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	} else {
		req.Header.Set("Content-Type", "application/octet-stream")
	}

	client := e.clientForUpstream()
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("license upstream error url=%s err=%v", targetURL, err)
		http.Error(w, "upstream license request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyHeaders(w, resp.Header)
	w.WriteHeader(resp.StatusCode)

	if r.Method != http.MethodHead && resp.Body != nil {
		_, _ = io.Copy(w, io.LimitReader(resp.Body, 1<<20))
	}
}

func (e *Engine) isHLSManifestURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	return strings.Contains(lower, ".m3u8") || strings.Contains(lower, "mpegurl")
}

func (e *Engine) acquire(ctx context.Context) error {
	select {
	case e.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *Engine) release() {
	<-e.sem
}

func (e *Engine) cacheKey(targetURL, auth string) string {
	return targetURL + "\x00" + auth
}

func (e *Engine) getCached(key string) ([]byte, bool) {
	if v, ok := e.cache.Load(key); ok {
		entry := v.(cacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.data, true
		}
		e.cache.Delete(key)
	}
	return nil, false
}

func (e *Engine) setCache(key string, data []byte, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	e.cache.Store(key, cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	})
}

func (e *Engine) serveDashManifest(w http.ResponseWriter, r *http.Request, targetURL, auth string) {
	key := e.cacheKey(targetURL, auth)
	if data, ok := e.getCached(key); ok {
		w.Header().Set("Content-Type", dashManifestContentType())
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
		return
	}

	body, status, sessionAuth, finalURL, err := e.fetchUpstream(r, targetURL, auth)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("dash manifest fetch error url=%s err=%v", targetURL, err)
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "upstream manifest fetch timed out", http.StatusGatewayTimeout)
			return
		}
		if strings.Contains(err.Error(), "exceeds limit") {
			http.Error(w, "manifest payload too large", http.StatusBadGateway)
			return
		}
		http.Error(w, "upstream fetch failed", http.StatusBadGateway)
		return
	}

	if status < 200 || status >= 300 {
		log.Printf("dash manifest upstream rejected url=%s status=%d", targetURL, status)
		writeUpstreamError(w, body, status)
		return
	}

	if !isValidDashManifest(body) {
		log.Printf("dash manifest invalid payload url=%s reason=missing_mpd", targetURL)
		http.Error(w, "upstream response is not a valid DASH manifest", http.StatusBadGateway)
		return
	}

	logUpstreamRedirect(targetURL, finalURL)

	effectiveAuth := utils.MergeAuth(auth, sessionAuth)
	rewritten, err := e.rewriteDashManifest(body, finalURL, effectiveAuth)
	if err != nil {
		log.Printf("dash manifest rewrite error url=%s err=%v", targetURL, err)
		http.Error(w, "manifest rewrite failed", http.StatusBadGateway)
		return
	}

	if dashManifestCacheAllowed(body) {
		e.setCache(key, rewritten, vodPlaylistCacheTTL)
	}

	w.Header().Set("Content-Type", dashManifestContentType())
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(rewritten)
	}
}

func (e *Engine) serveHLSManifest(w http.ResponseWriter, r *http.Request, targetURL, auth string) {
	key := e.cacheKey(targetURL, auth)
	if data, ok := e.getCached(key); ok {
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
		return
	}

	body, status, sessionAuth, finalURL, err := e.fetchUpstream(r, targetURL, auth)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("manifest fetch error url=%s err=%v", targetURL, err)
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "upstream manifest fetch timed out", http.StatusGatewayTimeout)
			return
		}
		if strings.Contains(err.Error(), "exceeds limit") {
			http.Error(w, "manifest payload too large", http.StatusBadGateway)
			return
		}
		http.Error(w, "upstream fetch failed", http.StatusBadGateway)
		return
	}

	logUpstreamRedirect(targetURL, finalURL)

	if status < 200 || status >= 300 {
		log.Printf("manifest upstream rejected url=%s status=%d", targetURL, status)
		writeUpstreamError(w, body, status)
		return
	}

	if !isHLSManifest(body) {
		log.Printf("manifest invalid payload url=%s reason=missing_extm3u", targetURL)
		http.Error(w, "upstream response is not a valid HLS manifest", http.StatusBadGateway)
		return
	}

	effectiveAuth := utils.MergeAuth(auth, sessionAuth)
	rewritten, err := e.rewriteManifest(body, finalURL, effectiveAuth)
	if err != nil {
		log.Printf("manifest rewrite error url=%s err=%v", targetURL, err)
		http.Error(w, "manifest rewrite failed", http.StatusBadGateway)
		return
	}

	if status >= 200 && status < 300 {
		if cacheTTL, ok := manifestCacheTTL(body); ok {
			e.setCache(key, rewritten, cacheTTL)
		}
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	if shouldBypassManifestCache(body) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(rewritten)
	}
}

func (e *Engine) streamBinary(w http.ResponseWriter, r *http.Request, targetURL, auth string) {
	resp, cancel, err := e.doUpstream(r, targetURL, auth, upstreamStream)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("upstream error url=%s err=%v", targetURL, err)
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "upstream stream timed out", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "upstream fetch failed", http.StatusBadGateway)
		return
	}
	defer cancel()
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		http.Error(w, string(snippet), resp.StatusCode)
		return
	}

	contentType := utils.ContentTypeForURL(targetURL, resp.Header.Get("Content-Type"))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")
	copyHeaders(w, resp.Header)

	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}

	written, copyErr := e.copyWithCancel(r.Context(), w, resp.Body)
	if copyErr != nil {
		e.logStreamCopyOutcome(r.Context(), targetURL, written, copyErr)
	}
}

func (e *Engine) fetchUpstream(r *http.Request, targetURL, auth string) ([]byte, int, string, string, error) {
	resp, cancel, err := e.doUpstream(r, targetURL, auth, upstreamManifest)
	if err != nil {
		return nil, 0, "", targetURL, err
	}
	defer cancel()
	defer resp.Body.Close()

	finalURL := upstreamFinalURL(resp, targetURL)
	sessionAuth := utils.SessionAuthFromHeaders(resp.Header)

	if resp.ContentLength > e.maxManifestSize {
		return nil, 0, sessionAuth, finalURL, fmt.Errorf("manifest content-length %d exceeds limit %d", resp.ContentLength, e.maxManifestSize)
	}

	limited := io.LimitReader(resp.Body, e.maxManifestSize+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, 0, sessionAuth, finalURL, err
	}
	if int64(len(body)) > e.maxManifestSize {
		return nil, 0, sessionAuth, finalURL, fmt.Errorf("manifest body exceeds limit %d bytes", e.maxManifestSize)
	}

	return body, resp.StatusCode, sessionAuth, finalURL, nil
}

func (e *Engine) doUpstream(r *http.Request, targetURL, auth string, kind upstreamKind) (*http.Response, context.CancelFunc, error) {
	ctx, cancel := e.upstreamContext(r, kind)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	upstreamAuth := utils.StripDrmAuth(auth)
	for k, values := range utils.HeadersForURL(targetURL, upstreamAuth) {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	if rng := r.Header.Get("Range"); rng != "" {
		req.Header.Set("Range", rng)
	}

	client := e.clientForUpstream()
	resp, err := client.Do(req)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return resp, cancel, nil
}

// clientForUpstream returns an HTTP client scoped to a single upstream fetch.
// Each client gets its own cookie jar so 302 redirect chains (Set-Cookie → tokenized URL)
// work like NS Player, without leaking cookies across unrelated streams.
func (e *Engine) clientForUpstream() *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		jar = nil
	}

	return &http.Client{
		Timeout:   e.client.Timeout,
		Transport: e.client.Transport,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= defaultMaxRedirectHops {
				return fmt.Errorf("stopped after %d redirects", defaultMaxRedirectHops)
			}
			// Preserve NS/playlist headers across redirect hops (token gates often drop them).
			if len(via) > 0 {
				inheritRedirectHeaders(req, via[0])
			}
			return nil
		},
	}
}

func inheritRedirectHeaders(next, first *http.Request) {
	for _, key := range []string{
		"User-Agent",
		"Referer",
		"Origin",
		"Cookie",
		"Authorization",
		"Accept-Encoding",
		"Accept",
		"Connection",
		"Icy-MetaData",
	} {
		if next.Header.Get(key) != "" {
			continue
		}
		if value := first.Header.Get(key); value != "" {
			next.Header.Set(key, value)
		}
	}

	for key, values := range first.Header {
		if !strings.HasPrefix(key, "X-") {
			continue
		}
		if next.Header.Get(key) != "" {
			continue
		}
		for _, value := range values {
			next.Header.Add(key, value)
		}
	}
}

func (e *Engine) upstreamContext(r *http.Request, kind upstreamKind) (context.Context, context.CancelFunc) {
	switch kind {
	case upstreamManifest:
		return context.WithTimeout(r.Context(), e.manifestTimeout)
	case upstreamStream:
		if e.streamTimeout > 0 {
			return context.WithTimeout(r.Context(), e.streamTimeout)
		}
		return r.Context(), func() {}
	default:
		return r.Context(), func() {}
	}
}

func (e *Engine) logStreamCopyOutcome(ctx context.Context, targetURL string, written int64, err error) {
	switch {
	case isClientDisconnect(ctx, err):
		log.Printf("stream client disconnect url=%s bytes_sent=%d", targetURL, written)
	case isUpstreamConnectionReset(err):
		log.Printf("stream upstream connection reset url=%s bytes_sent=%d err=%v", targetURL, written, err)
	default:
		log.Printf("stream copy error url=%s bytes_sent=%d err=%v", targetURL, written, err)
	}
}

func isClientDisconnect(ctx context.Context, err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	if ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "broken pipe")
}

func isUpstreamConnectionReset(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNRESET) || errors.Is(opErr.Err, syscall.EPIPE) {
			return true
		}
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "forcibly closed by the remote host") ||
		strings.Contains(msg, "use of closed network connection")
}

// copyWithCancel streams data to the client and stops when the client disconnects.
func (e *Engine) copyWithCancel(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	if ctx.Done() == nil {
		return io.Copy(dst, src)
	}

	type copyResult struct {
		n   int64
		err error
	}

	done := make(chan copyResult, 1)
	go func() {
		n, err := io.Copy(dst, src)
		done <- copyResult{n: n, err: err}
	}()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case res := <-done:
		return res.n, res.err
	}
}

func (e *Engine) rewriteManifest(body []byte, baseURL, auth string) ([]byte, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	reader := bufio.NewReader(bytes.NewReader(body))

	for {
		line, err := readManifestLine(reader, e.maxManifestLine)
		if errors.Is(err, io.EOF) {
			break
		}
		if errors.Is(err, errManifestLineTooLong) {
			log.Printf("manifest skip malformed line url=%s reason=line_exceeds_buffer", baseURL)
			continue
		}
		if err != nil {
			return nil, err
		}

		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if strings.HasPrefix(strings.ToUpper(trimmed), "#EXT-X-STREAM-INF") {
				out.WriteString(line)
				out.WriteByte('\n')
				continue
			}
			if strings.Contains(strings.ToUpper(trimmed), "URI=") {
				out.WriteString(e.rewriteTagURI(line, base, auth))
			} else {
				out.WriteString(line)
			}
			out.WriteByte('\n')
			continue
		}

		resolved := resolveReference(base, trimmed)
		out.WriteString(e.proxyURL(resolved, auth))
		out.WriteByte('\n')
	}

	return out.Bytes(), nil
}

func readManifestLine(r *bufio.Reader, maxLen int) (string, error) {
	var buf bytes.Buffer
	buf.Grow(256)

	for {
		b, err := r.ReadByte()
		if err == io.EOF {
			if buf.Len() == 0 {
				return "", io.EOF
			}
			return buf.String(), nil
		}
		if err != nil {
			return "", err
		}

		if b == '\n' {
			return strings.TrimRight(buf.String(), "\r"), nil
		}

		if buf.Len() >= maxLen {
			if skipErr := skipUntilNewline(r); skipErr != nil && !errors.Is(skipErr, io.EOF) {
				return "", skipErr
			}
			return "", errManifestLineTooLong
		}

		buf.WriteByte(b)
	}
}

func skipUntilNewline(r *bufio.Reader) error {
	for {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		if b == '\n' {
			return nil
		}
	}
}

func (e *Engine) rewriteTagURI(line string, base *url.URL, auth string) string {
	const key = "URI=\""
	upper := strings.ToUpper(line)
	idx := strings.Index(upper, key)
	if idx < 0 {
		return line
	}

	start := idx + len(key)
	end := strings.Index(line[start:], "\"")
	if end < 0 {
		return line
	}

	uri := line[start : start+end]
	resolved := resolveReference(base, uri)
	proxied := e.proxyURL(resolved, auth)

	return line[:start] + proxied + line[start+end:]
}

func resolveReference(base *url.URL, ref string) string {
	parsed, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return base.ResolveReference(parsed).String()
}

func (e *Engine) proxyURL(target, auth string) string {
	if e.tokenSecret != "" {
		token, err := utils.EncryptPlayToken(e.tokenSecret, target, auth, 6*time.Hour)
		if err == nil {
			return fmt.Sprintf("%s/proxy?t=%s", e.proxyBase, url.QueryEscape(token))
		}
	}

	out := fmt.Sprintf("%s/proxy?url=%s", e.proxyBase, url.QueryEscape(target))
	if auth != "" {
		out += "&auth=" + url.QueryEscape(auth)
	}
	return out
}

func copyHeaders(w http.ResponseWriter, src http.Header) {
	// Whitelist only safe response headers. Transfer-Encoding and Connection are
	// hop-by-hop and must not be forwarded — they would conflict with the proxy transport.
	for _, k := range []string{"Accept-Ranges", "Content-Length", "Content-Range"} {
		if v := src.Get(k); v != "" {
			w.Header().Set(k, v)
		}
	}
}

// ActiveStreams returns the number of in-flight proxy connections.
func (e *Engine) ActiveStreams() int {
	return len(e.sem)
}

// MaxConcurrent returns the configured concurrency limit.
func (e *Engine) MaxConcurrent() int {
	return e.maxConcurrent
}

// manifestCacheTTL decides whether a rewritten manifest may be cached.
// Live media playlists must never be cached — they gain new segments continuously.
func manifestCacheTTL(body []byte) (time.Duration, bool) {
	content := string(body)

	if strings.Contains(content, "#EXT-X-ENDLIST") {
		return vodPlaylistCacheTTL, true
	}

	if strings.Contains(content, "#EXT-X-STREAM-INF:") && !strings.Contains(content, "#EXTINF:") {
		return masterPlaylistCacheTTL, true
	}

	return 0, false
}

func shouldBypassManifestCache(body []byte) bool {
	_, ok := manifestCacheTTL(body)
	return !ok
}

func isHLSManifest(body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	return strings.HasPrefix(trimmed, "#EXTM3U")
}

func writeUpstreamError(w http.ResponseWriter, body []byte, status int) {
	snippet := strings.TrimSpace(string(body))
	if snippet == "" {
		http.Error(w, http.StatusText(status), status)
		return
	}
	if len(snippet) > 512 {
		snippet = snippet[:512]
	}
	http.Error(w, snippet, status)
}

func logUpstreamRedirect(requestedURL, finalURL string) {
	if requestedURL == "" || finalURL == "" || requestedURL == finalURL {
		return
	}
	log.Printf("upstream redirect requested=%s final=%s", requestedURL, finalURL)
}

func upstreamFinalURL(resp *http.Response, requestedURL string) string {
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		return resp.Request.URL.String()
	}
	return requestedURL
}

func isUpstreamHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}
