package har

// HAR represents the complete HAR (HTTP Archive) format as per HAR 1.2 spec
type HAR struct {
	Log *Log `json:"log"`
}

// Log contains the HTTP archive data
type Log struct {
	Version string   `json:"version"`
	Creator *Creator `json:"creator"`
	Browser *Browser `json:"browser,omitempty"`
	Pages   []*Page  `json:"pages,omitempty"`
	Entries []*Entry `json:"entries"`
	Comment string   `json:"comment,omitempty"`
}

// Creator describes the application that created the archive
type Creator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Comment string `json:"comment,omitempty"`
}

// Browser describes the browser that created the archive (optional)
type Browser struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Comment string `json:"comment,omitempty"`
}

// Page describes a page within the archive (optional)
type Page struct {
	ID              string       `json:"id"`
	StartedDateTime string       `json:"startedDateTime"`
	Title           string       `json:"title"`
	PageTimings     *PageTimings `json:"pageTimings"`
	Comment         string       `json:"comment,omitempty"`
}

// PageTimings describes timings for a page
type PageTimings struct {
	OnContentLoad float64 `json:"onContentLoad,omitempty"`
	OnLoad        float64 `json:"onLoad,omitempty"`
	Comment       string  `json:"comment,omitempty"`
}

// Entry describes a single HTTP request/response pair
type Entry struct {
	PageRef         string    `json:"pageref,omitempty"`
	StartedDateTime string    `json:"startedDateTime"`
	Time            float64   `json:"time"`
	Request         *Request  `json:"request"`
	Response        *Response `json:"response"`
	Cache           *Cache    `json:"cache"`
	Timings         *Timings  `json:"timings"`
	ServerIPAddress string    `json:"serverIPAddress,omitempty"`
	Connection      string    `json:"connection,omitempty"`
	Comment         string    `json:"comment,omitempty"`
}

// Request describes an HTTP request
type Request struct {
	Method      string         `json:"method"`
	URL         string         `json:"url"`
	HTTPVersion string         `json:"httpVersion"`
	Headers     []*Header      `json:"headers"`
	QueryString []*QueryString `json:"queryString"`
	Cookies     []*Cookie      `json:"cookies"`
	PostData    *PostData      `json:"postData,omitempty"`
	HeadersSize int            `json:"headersSize"`
	BodySize    int            `json:"bodySize"`
	Comment     string         `json:"comment,omitempty"`
}

// Response describes an HTTP response
type Response struct {
	Status      int       `json:"status"`
	StatusText  string    `json:"statusText"`
	HTTPVersion string    `json:"httpVersion"`
	Headers     []*Header `json:"headers"`
	Cookies     []*Cookie `json:"cookies"`
	Content     *Content  `json:"content"`
	RedirectURL string    `json:"redirectURL"`
	HeadersSize int       `json:"headersSize"`
	BodySize    int       `json:"bodySize"`
	Comment     string    `json:"comment,omitempty"`
}

// Header represents an HTTP header as a name-value pair
type Header struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Comment string `json:"comment,omitempty"`
}

// QueryString represents a URL query parameter
type QueryString struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Comment string `json:"comment,omitempty"`
}

// Cookie represents an HTTP cookie
type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Path     string `json:"path,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Expires  string `json:"expires,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

// PostData describes POST request data
type PostData struct {
	MimeType string       `json:"mimeType"`
	Params   []*PostParam `json:"params,omitempty"`
	Text     string       `json:"text,omitempty"`
	Comment  string       `json:"comment,omitempty"`
}

// PostParam represents a parameter in POST data
type PostParam struct {
	Name        string `json:"name"`
	Value       string `json:"value,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

// Content describes the response body content
type Content struct {
	Size        int    `json:"size"`
	Compression int    `json:"compression,omitempty"`
	MimeType    string `json:"mimeType"`
	Text        string `json:"text,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

// Cache describes cache status information
type Cache struct {
	BeforeRequest *CacheState `json:"beforeRequest,omitempty"`
	AfterRequest  *CacheState `json:"afterRequest,omitempty"`
	Comment       string      `json:"comment,omitempty"`
}

// CacheState describes the state of a cache entry
type CacheState struct {
	Expires    string `json:"expires,omitempty"`
	LastAccess string `json:"lastAccess,omitempty"`
	ETag       string `json:"eTag,omitempty"`
	HitCount   int    `json:"hitCount,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

// Timings describes timing information for an entry
type Timings struct {
	Blocked float64 `json:"blocked,omitempty"`
	DNS     float64 `json:"dns,omitempty"`
	Connect float64 `json:"connect,omitempty"`
	Send    float64 `json:"send,omitempty"`
	Wait    float64 `json:"wait,omitempty"`
	Receive float64 `json:"receive,omitempty"`
	SSL     float64 `json:"ssl,omitempty"`
	Comment string  `json:"comment,omitempty"`
}
