package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

type pageSender func(context.Context, string, string, any, any) (http.Header, error)
type nextPageFunc func(string, http.Header) string

func fetchPages(ctx context.Context, path string, out any, send pageSender, next nextPageFunc) error {
	outValue := reflect.ValueOf(out)
	if outValue.Kind() != reflect.Pointer || outValue.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("pagination: output must be pointer to slice")
	}
	slice := outValue.Elem()
	page := path
	for page != "" {
		pagePtr := reflect.New(slice.Type())
		headers, err := send(ctx, http.MethodGet, page, nil, pagePtr.Interface())
		if err != nil {
			return err
		}
		slice.Set(reflect.AppendSlice(slice, pagePtr.Elem()))
		page = next(page, headers)
	}
	return nil
}

func nextGitHubPage(_ string, headers http.Header) string {
	for _, part := range strings.Split(headers.Get("Link"), ",") {
		segments := strings.Split(part, ";")
		if len(segments) < 2 {
			continue
		}
		link := strings.TrimSpace(segments[0])
		if !strings.HasPrefix(link, "<") || !strings.HasSuffix(link, ">") {
			continue
		}
		for _, attr := range segments[1:] {
			if strings.TrimSpace(attr) == `rel="next"` {
				return strings.TrimSuffix(strings.TrimPrefix(link, "<"), ">")
			}
		}
	}
	return ""
}

func nextGitLabPage(current string, headers http.Header) string {
	nextPage := strings.TrimSpace(headers.Get("X-Next-Page"))
	if nextPage == "" {
		return ""
	}
	parsed, err := url.Parse(current)
	if err != nil {
		return ""
	}
	values := parsed.Query()
	values.Set("page", nextPage)
	if values.Get("per_page") == "" {
		values.Set("per_page", "100")
	}
	parsed.RawQuery = values.Encode()
	return parsed.String()
}
