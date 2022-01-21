package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/88250/lute"
	"github.com/PuerkitoBio/goquery"
)

var (
	// ErrNotFoundLastModified 响应请求没有LastModified头
	ErrNotFoundLastModified = errors.New("not found last modified")
)

func newDoc(path string) (*goquery.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	html := lute.New().Md2HTML(string(data))
	return goquery.NewDocumentFromReader(strings.NewReader(html))
}

// 检查软件镜像
func checkPackage(path string) {
	doc, err := newDoc(path)
	if err != nil {
		panic(err)
	}
	// var officialContentLength int64
	// var officialLastModified *time.Time
	// doc.Find("a").Each(func(i int, s *goquery.Selection) {
	// 	href := s.AttrOr("href", "")
	// 	if !strings.Contains(href, "deepin.com") {
	// 		return
	// 	}
	// 	if officialContentLength == 0 {
	// 		officialContentLength, officialLastModified, err = head(inReleaseURL(href))
	// 		if err != nil {
	// 			panic(err)
	// 		}
	// 	}
	// })
	doc.Find("table").First().Find("a").Each(func(_ int, s *goquery.Selection) {
		href := s.AttrOr("href", "")
		if !strings.HasPrefix(href, "http") {
			// 禁止不支持的协议标记
			return
			fmt.Printf("/* url: %s msg: Unsupported protocol */\n", href)
			fmt.Println(cssA(href, "black", "lightgrey"))
			return
		}
		contentLength, lastModified, err := head(inReleaseURL(href))
		if err != nil {
			log.Println(href, "失败", err)
			fmt.Printf("/* url: %s msg: %s */\n", href, err)
			fmt.Println(cssA(href, "white", "red"))
			fmt.Println()
			return
		}
		_ = contentLength
		_ = lastModified
		// if contentLength != officialContentLength {
		// 	if officialLastModified.Sub(*lastModified) > time.Hour*24*7 {
		// 		log.Println(href, "过时")
		// 		fmt.Printf("/* url: %s msg: Outdated */\n", href)
		// 		fmt.Println(cssA(href, "black", "yellow"))
		// 		fmt.Println()
		// 		return
		// 	}
		// }
	})
}

// 检查ISO镜像源
func checkRelease(path string) {
	doc, err := newDoc(path)
	if err != nil {
		panic(err)
	}
	doc.Find("table").First().Find("a").Each(func(_ int, s *goquery.Selection) {
		href := s.AttrOr("href", "")
		if !strings.HasPrefix(href, "http") {
			fmt.Printf("/* url: %s msg: Unsupported protocol */\n", href)
			fmt.Println(cssA(href, "black", "lightgrey"))
			return
		}
		_, _, err := head(href)
		if err != nil {
			if errors.Is(err, ErrNotFoundLastModified) {
				return
			}
			log.Println(href, "失败", err)
			fmt.Printf("/* url: %s msg: %s */\n", href, err)
			fmt.Println(cssA(href, "white", "red"))
			fmt.Println()
			return
		}
	})
}

func main() {
	var packagePATH, releasePATH string
	flag.StringVar(&packagePATH, "packages_path", "", "package mirrors markdown file path")
	flag.StringVar(&releasePATH, "releases_path", "", "release mirrors markdown file path")
	flag.Parse()
	switch {
	case len(packagePATH) > 0:
		checkPackage(packagePATH)
	case len(releasePATH) > 0:
		checkRelease(releasePATH)
	default:
		flag.PrintDefaults()
	}
}

// 发送HEAD请求
func head(url string) (ContentLength int64, LastModified *time.Time, err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
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

// 生成css标记
func cssA(href string, color, bgColor string) string {
	return fmt.Sprintf(`a[href="%s"] { padding: 3px; color: %s; background-color: %s; }`, href, color, bgColor)
}
