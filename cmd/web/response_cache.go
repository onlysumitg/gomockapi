package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/onlysumitg/GoMockAPI/utils/xmlutils"
)

var cachedDelayRangeRegex = regexp.MustCompile(`(?m)\((?P<Start>\d+),(?P<End>\d*)\)`)

type cachedResponse struct {
	StatusCode     int
	Body           string
	ResponseType   string
	ResponseHeader http.Header
	DelayValue     string // raw delay: "500" or "(100,1000)"
}

// applyCachedDelay sleeps for the configured delay, handling both fixed
// and (start,end) range values.
func (cr *cachedResponse) applyCachedDelay() {
	if cr.DelayValue == "" {
		return
	}
	intval, err := strconv.Atoi(cr.DelayValue)
	if err == nil {
		time.Sleep(time.Duration(intval) * time.Millisecond)
		return
	}
	start := cachedDelayRangeRegex.SubexpIndex("Start")
	end := cachedDelayRangeRegex.SubexpIndex("End")
	matches := cachedDelayRangeRegex.FindStringSubmatch(cr.DelayValue)
	if len(matches) > 0 {
		startInt, err1 := strconv.Atoi(matches[start])
		if err1 != nil || startInt <= 0 {
			startInt = 1
		}
		endInt, err2 := strconv.Atoi(matches[end])
		if err2 != nil || endInt <= 0 || endInt <= startInt {
			endInt = startInt + (10 * 1000)
		}
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		delay := r.Intn(endInt-startInt+1) + startInt
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
}

// buildResponseCacheKey builds a cache key from the endpoint key and the
// request values that conditions inspect. Keys are sorted for stability.
func buildResponseCacheKey(endpointKey string, conditionKeys []string, requestFlatMap map[string]xmlutils.ValueDatatype) string {
	var b strings.Builder
	b.WriteString(endpointKey)

	sorted := make([]string, len(conditionKeys))
	copy(sorted, conditionKeys)
	sort.Strings(sorted)

	for _, k := range sorted {
		b.WriteByte('|')
		b.WriteString(k)
		b.WriteByte('=')
		if v, ok := requestFlatMap[k]; ok {
			b.WriteString(fmt.Sprintf("%v", v.Value))
		}
	}
	return b.String()
}

// getResponseCache looks up a cached response for the given endpoint and request.
func (app *application) getResponseCache(cacheKey string) (*cachedResponse, bool) {
	v, ok := app.responseCache.Load(cacheKey)
	if !ok {
		return nil, false
	}
	cr, ok := v.(*cachedResponse)
	return cr, ok
}

// setResponseCache stores a computed response in the cache.
func (app *application) setResponseCache(cacheKey string, cr *cachedResponse) {
	app.responseCache.Store(cacheKey, cr)
}

// clearResponseCache wipes the entire response cache.
// Called alongside endpoint cache invalidation.
func (app *application) clearResponseCache() {
	app.responseCache = sync.Map{}
}
