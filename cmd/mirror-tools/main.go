package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"golang.org/x/sync/errgroup"
)

var (
	// ErrNotFoundLastModified 响应请求没有LastModified头
	ErrNotFoundLastModified = errors.New("not found last modified")
)

// MirrorSource Copy from linuxdeepin/lastore-daemon/src/internal/system/common.go
type MirrorSource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`

	NameLocale  map[string]string `json:"name_locale"`
	Weight      int               `json:"weight"`
	Country     string            `json:"country"`
	AdjustDelay int               `json:"adjust_delay"` // ms
}

// ParseMirrorSource parse mirror source file
func ParseMirrorSource(data []byte) ([]MirrorSource, error) {
	var ms []MirrorSource
	err := json.Unmarshal(data, &ms)
	if err != nil {
		return nil, fmt.Errorf("unmarshal mirror source file: %w", err)
	}
	return ms, nil
}
func main() {
	var f string
	var timeout time.Duration
	flag.StringVar(&f, "f", "mirrors.json", "mirrors file")
	flag.DurationVar(&timeout, "timeout", time.Second*5, "request timeout")
	flag.Parse()
	err := runMirrorCheck(context.Background(), f, timeout)
	if err != nil {
		log.Fatal(err)
	}
}

func runMirrorCheck(ctx context.Context, mirrorSourceFile string, timeout time.Duration) error {
	data, err := os.ReadFile(mirrorSourceFile)
	if err != nil {
		return fmt.Errorf("read mirror source file: %w", err)
	}
	ms, err := ParseMirrorSource(data)
	if err != nil {
		return err
	}
	check := func(m MirrorSource) error {
		return retry.Do(func() error {
			href := m.URL
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			_, _, err = head(ctx, inReleaseURL(href))
			if err != nil {
				log.Println(m.Name, err, m.URL)
				return err
			}
			return err
		})
	}
	var eg errgroup.Group
	var ch = make(chan struct{}, runtime.NumCPU())
	for i := range ms {
		m := ms[i]
		ch <- struct{}{}
		eg.Go(func() error {
			defer func() {
				<-ch
			}()
			return check(m)
		})
	}
	return eg.Wait()
}

// 发送HEAD请求
func head(ctx context.Context, url string) (ContentLength int64, LastModified *time.Time, err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.159 Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, nil, errors.New(resp.Status)
	}
	contentLength := resp.ContentLength
	lastModifiedStr := resp.Header.Get("Last-Modified")
	if len(lastModifiedStr) == 0 {
		return 0, nil, ErrNotFoundLastModified
	}
	t, err := time.Parse(http.TimeFormat, lastModifiedStr)
	if err != nil {
		return 0, nil, err
	}
	return contentLength, &t, nil
}

func inReleaseURL(href string) string {
	return strings.Trim(href, "/") + "/dists/apricot/InRelease"
}
