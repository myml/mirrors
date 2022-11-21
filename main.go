package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/88250/lute"
	"github.com/PuerkitoBio/goquery"
)

var (
	// ErrNotFoundLastModified 响应请求没有LastModified头
	ErrNotFoundLastModified = errors.New("not found last modified")
	// ErrUnknownContentType 未知的文件类型
	ErrUnknownContentType = errors.New("unknown content type")
)

var HeadContentType = "application/octet-stream"
var Timeout = time.Second * 10

func main() {
	var packagePATH, releasePATH, checkFile string
	flag.StringVar(&packagePATH, "packages_path", "", "package mirrors markdown file path")
	flag.StringVar(&releasePATH, "releases_path", "", "release mirrors markdown file path")
	flag.StringVar(&checkFile, "check_file", "", "check file path")
	flag.StringVar(&HeadContentType, "content_type", HeadContentType, "response content type")
	flag.DurationVar(&Timeout, "timeout", Timeout, "request timeout")
	flag.Parse()
	switch {
	case len(packagePATH) > 0:
		checkPackage(packagePATH, checkFile)
	case len(releasePATH) > 0:
		checkRelease(releasePATH, checkFile)
	default:
		flag.PrintDefaults()
	}
}

func newDoc(path string) (*goquery.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	html := lute.New().Md2HTML(string(data))
	return goquery.NewDocumentFromReader(strings.NewReader(html))
}

// 检查软件镜像
func checkPackage(path, checkFile string) {
	doc, err := newDoc(path)
	if err != nil {
		panic(err)
	}
	hrefList := make(map[string]struct{})
	doc.Find("table").First().Find("a").Each(func(_ int, s *goquery.Selection) {
		href := s.AttrOr("href", "")
		hrefList[href] = struct{}{}
	})

	// 生成css
	errChan := make(chan [2]string)
	defer close(errChan)
	go func() {
		for v := range errChan {
			href, err := v[0], v[1]
			fmt.Printf("/* url: %s msg: %s */\n", href, err)
			fmt.Println(cssA(href, "white", "red"))
			fmt.Println()
		}
	}()
	// 检查
	var limitChan = make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for href := range hrefList {
		limitChan <- struct{}{}
		wg.Add(1)
		go func(href string) {
			defer func() {
				wg.Done()
				<-limitChan
			}()
			if !strings.HasPrefix(href, "http") {
				log.Println(href, "不支持")
				return
			}
			_, _, err := head(strings.Trim(href, "/") + checkFile)
			if err != nil {
				log.Println(href, "失败", err)
				errChan <- [2]string{href, err.Error()}
				return
			}
			log.Println(href, "有效")
		}(href)
	}
	wg.Wait()
}

// 检查ISO镜像源
func checkRelease(path, checkFile string) {
	doc, err := newDoc(path)
	if err != nil {
		panic(err)
	}
	hrefList := make(map[string]struct{})
	doc.Find("table").First().Find("a").Each(func(_ int, s *goquery.Selection) {
		href := s.AttrOr("href", "")
		hrefList[href] = struct{}{}
	})

	// 生成css
	errChan := make(chan [2]string)
	defer close(errChan)
	go func() {
		for v := range errChan {
			href, err := v[0], v[1]
			fmt.Printf("/* url: %s msg: %s */\n", href, err)
			fmt.Println(cssA(href, "white", "red"))
			fmt.Println()
		}
	}()

	// 检查
	var limitChan = make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup

	for href := range hrefList {
		limitChan <- struct{}{}
		wg.Add(1)
		go func(href string) {
			defer func() {
				wg.Done()
				<-limitChan
			}()
			if !strings.HasPrefix(href, "http") {
				log.Println(href, "不支持")
				return
			}
			_, _, err := head(strings.Trim(href, "/") + checkFile)
			if err != nil {
				log.Println(href, "失败", err)
				errChan <- [2]string{href, err.Error()}
				return
			}
			log.Println(href, "有效")
		}(href)
	}
}

// 发送HEAD请求
func head(url string) (ContentLength int64, LastModified *time.Time, err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
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
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/octet-stream" {
		return 0, nil, ErrNotFoundLastModified
	}
	t, err := time.Parse(http.TimeFormat, lastModifiedStr)
	if err != nil {
		return 0, nil, err
	}
	return contentLength, &t, nil
}

// 生成css标记
func cssA(href string, color, bgColor string) string {
	return fmt.Sprintf(`a[href="%s"] { padding: 3px; color: %s; background-color: %s; }`, href, color, bgColor)
}
