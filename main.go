/* REQUIREMENTS
// Create a sitemap for a user specified url
// Only include URL's in same domain
// Output sitemap in standard sitemap protocol XML
*/

package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/PhamJamStudio/link"
)

const xmlns = "http://www.sitemaps.org/schemas/sitemap/0.9"

type loc struct {
	Val string `xml:"loc"`
}

type urlset struct {
	Urls  []loc  `xml:"url"`
	Xmlns string `xml:"xmlns,attr"`
}

func main() {
	urlFlag := flag.String("url", "https://eqmac.app/", "Domain URL for sitemap creation")
	maxDepth := flag.Int("depth", 3, "Max # of links deep to traverse")
	flag.Parse()

	// DL HTML from URL, get base domain, filter and return urls via BFS
	pages := bfs(*urlFlag, *maxDepth)

	//  print out XML
	toXML := urlset{Urls: make([]loc, len(pages)),
		Xmlns: xmlns}
	for i, page := range pages {
		toXML.Urls[i] = loc{page}
	}
	fmt.Print(xml.Header)
	enc := xml.NewEncoder(os.Stdout)
	enc.Indent("", "   ")
	if err := enc.Encode(toXML); err != nil {
		fmt.Printf("error: %v\n", err)
	}

}

/* bfs creates two queues to track the current list of domains to extract URL's from, then another queue to track the next set of domains to extract URL's from. We repeat this process to a user specified depth and ignore duplicate URL's and links to other domains
 */
func bfs(urlStr string, maxDepth int) []string {
	// Keep track of pages seen. Can use bool, but struct uses a little less mem -> https://dave.cheney.net/2014/03/25/the-empty-struct
	seen := make(map[string]struct{})

	// queue we're currently looking at
	var q map[string]struct{}

	// Next queue receives all unseen links from above queue urls. Once we've finishing going thru q, we reassign nq to q and repeat loop
	nq := map[string]struct{}{
		urlStr: {},
	}
	for i := 0; i <= maxDepth; i++ {
		q, nq = nq, make(map[string]struct{})
		//  Break if there are no url's in queue
		if len(q) == 0 {
			break
		}
		for url := range q {
			// skip if url has already been seen
			if _, ok := seen[url]; ok {
				continue
			}
			// Extract and review all links in current url
			for _, link := range getDomainPages(url) {
				// only add link to nq if it hasn't already been seen
				if _, ok := seen[link]; !ok {
					nq[link] = struct{}{}
				}
			}
			// Mark url as seen once we traverse all its links
			fmt.Println("URL found:", url)
			seen[url] = struct{}{}
		}
	}
	// return list of strings seen
	ret := make([]string, 0, len(seen))
	for url := range seen {
		ret = append(ret, url)
	}
	return ret
}

// Given a URL, DL HTML, get domain, return links belonging to specified domain
func getDomainPages(urlStr string) []string {
	resp, err := http.Get(urlStr)
	if err != nil {
		log.Fatalln("ERROR:", err)
	}
	defer resp.Body.Close()

	// Build proper urls w/ our links i.e. add scheme (http, https, ftp, etc) + host = domain, ignore mailto, different domains
	base := getDomain(resp.Request.URL)
	return filterURLs(getURLs(resp.Body, base), withPrefix(base))
}

// Given a URL, return the domain e.g https://eqmac.app
func getDomain(u *url.URL) string {
	// url.Parse() -> u.Host doesn't include the redir url, which other links would likely use
	baseURL := &url.URL{ // create new URL since we only need scheme/host
		Scheme: u.Scheme, // Scheme includes ftp, http, https, etc.
		Host:   u.Host,
	}
	return baseURL.String() // returns non nil fields in URL as concatenated string, here just scheme e.g HTTPS, and Host, e.g https://eqmac.app
}

// Given a http.response.body representing HTML from URL, return all URLs
func getURLs(h io.Reader, base string) []string {
	// Parse all links on the page
	links, err := link.Parse(h)
	if err != nil {
		log.Fatalln("ERROR:", err)
	}

	// Add sanitized URL's to links to return
	var ret []string
	for _, l := range links {
		switch {
		// if href doesn't include domain, add it before appending
		case strings.HasPrefix(l.Href, "/"):
			ret = append(ret, base+l.Href)
		// if href starts with http, append as is, which includes https
		case strings.HasPrefix(l.Href, "http"):
			// TODO: Don't add dupes
			ret = append(ret, l.Href)
		}
	}

	return ret
}

// Given a list of URLs, returned filtered list of URL's based on keepFn. which is a func w/ filter criteria
func filterURLs(links []string, keepFn func(string) bool) []string {
	var ret []string
	for _, link := range links {
		if keepFn(link) { // excludes everything not the same as base domain including mailto:, other domains
			ret = append(ret, link)
		}
	}
	return ret
}

// Used to filter out links that don't have a specified prefix
func withPrefix(pfx string) func(string) bool {
	return func(link string) bool {
		return strings.HasPrefix(link, pfx)
	}
}

/* LEARNING NOTES
// HTTP
	// http.Get(url), resp.Body includes content
	// URL.Scheme includes ftp, http, https, etc., so url.URL.String() can you get you https://www.theverge.com

// General
	// BFS: add top layer to queue, add their childrewn to next queue, repeat for children, etc
	// Empty structs use less mem than bools and can be used as map vals
	// io.Copy from reader to writer
	// strings.HasPrefix can check if the beginning of a string == prefix
	// Define and init empty struct i.e moo := struct{}{}
	// Terminal: go run main.go > map.xml outputs os.stdout to file

// XML
	// decoders behave similarly as JSON/YAML
	// 	Xmlns string `xml:"xmlns,attr"`, where attr shows up as a attribute of e.g urlset -> <urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
*/

/* ENHANCEMENT IDEAS / bugs
// How to optimize / speed up perf? Try getting snapshow of stats every 100ms. What's the bottleneck, is worth optimizing e.g dl pages, parsing HTML, compute speed vs n/w & I/O speed. How to stop waiting to max constantly compute?
	// Look into increasing parallelism e.g dl multiple pages, consider latency (high = expensive), try batch requests
	// Look into perf measuring / profiler to see where prog is spending most of the time. Can pre-pend timer in prog, in funcz

// Link management
	// Handle broken links gracefully
	// Remove similar URLS e.g www.google.com/ and www.google.com
	// Ignore non sites e.g links to bugs

// Misc
	// How to test/verify site maps of typical huge sites e.g theverge, https://www.apple.com/sitemap/
	// Better logs to show if we're at depth 1, 2, etc.
	// Add test code, compare to existing site maps
	// output to csv
*/

/* DESIGN NOTES
// Get user submitted URL
// BFS get all links within given domain up to BFS depth (aka # of clicks)
	// Track seen links via map[string]struct{}
	// Create current queue, init next queue with user submitted url
	// assign next queue to current queue, create new next queue
		// Start loop up to maxDepth
			// assign next queue to queue, make new next queue to receive links from current queue links
			// Base case: If len(queue) = 0, break loop
			// For each url in current queue
				// if seen, continue to next link
				// Get child links getDomainPages()
					// NOTE: each link needs to be sanitized e.g add domain, ignore if diff domain/mailto:, or other conditions in current queue
					// If link already seen, skip
				// Add links to next queue
				// Ignore irrelevant links e.g mailto:, diff domain
	// BFS Helper funcs
		// getDomainPages: Given link, return list of urls
			// Download URL html page via http.get
			// get URL domain
			// base := getDomain(resp.Request.URL)
			// return filterURLs(getURLs(resp.Body, base), withPrefix(base))
			// use link.Parse(url) to get hrefs given url's
			// For list of hrefs
				// If hrefs start w/ different domain or mailto:, ignore
				// add domain to href path if needed
				// add URL to list of URL's to return
		// getDomain
		// getURLs: get hrefs from link.Parse() add base url or ignore link as needed
		// withPrefix()
//  Convert filtered pages into to XML friendly format i.e. loc struct w/ string for url, urlset struct w/ list of loc's & optional Xmlns attr
	// Create xml encoder w/ stdout, encode toXML into xml and write into stdout

*/
