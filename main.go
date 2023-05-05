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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/88250/lute"
	"github.com/PuerkitoBio/goquery"
	"github.com/samber/lo"
)

var (
	// ErrNotFoundLastModified 响应请求没有LastModified头
	ErrNotFoundLastModified = errors.New("not found last modified")
)

var Timeout = time.Second * 10

func main() {
	var sourceURL, packagePATH, releasePATH, checkFile string
	flag.StringVar(&sourceURL, "source", "", "source url")
	flag.StringVar(&packagePATH, "packages_path", "", "package mirrors markdown file path")
	flag.StringVar(&releasePATH, "releases_path", "", "release mirrors markdown file path")
	flag.StringVar(&checkFile, "check_file", "", "check file path")
	flag.DurationVar(&Timeout, "timeout", Timeout, "request timeout")
	flag.Parse()
	switch {
	case len(packagePATH) > 0:
		links, err := getMarkdownLinks(packagePATH)
		if err != nil {
			log.Fatal(err)
		}
		checkMirror(sourceURL, checkFile, links)
	case len(releasePATH) > 0:
		links, err := getMarkdownLinks(releasePATH)
		if err != nil {
			log.Fatal(err)
		}
		checkMirror(sourceURL, checkFile, links)
	default:
		flag.PrintDefaults()
	}
}

func getMarkdownLinks(mdPath string) ([]string, error) {
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, err
	}
	html := lute.New().Md2HTML(string(data))
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	hrefList := make(map[string]struct{})
	doc.Find("table").First().Find("a").Each(func(_ int, s *goquery.Selection) {
		href := s.AttrOr("href", "")
		hrefList[href] = struct{}{}
	})
	hrefs := lo.Keys(hrefList)
	sort.Strings(hrefs)
	return hrefs, nil
}

// 检查软件镜像
func checkMirror(source, checkFile string, mirrors []string) {
	var errStore sync.Map
	// 获取源文件大小
	sourceLength, _, err := head(strings.Trim(source, "/") + checkFile)
	if err != nil {
		log.Fatal(err)
	}
	// 检查镜像文件是否等于源文件大小
	var limitChan = make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for _, href := range mirrors {
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
			mirrorLength, _, err := head(strings.Trim(href, "/") + checkFile)
			if err != nil {
				log.Println(href, "失败", err)
				errStore.Store(href, err.Error())
				return
			}
			if mirrorLength != sourceLength {
				log.Println(href, "失败", "长度不一致")
				errStore.Store(href, "长度不一致")
				return
			}
			log.Println(href, "有效")
		}(href)
	}
	wg.Wait()
	// 标记有问题的镜像
	for _, href := range mirrors {
		v, ok := errStore.Load(href)
		if !ok {
			continue
		}
		err := v.(string)
		fmt.Printf("/* url: %s msg: %s */\n", href, err)
		fmt.Println(cssA(href, "white", "gray"))
		fmt.Println()
	}
}

// 发送HEAD请求
func head(url string) (ContentLength int64, LastModified *time.Time, err error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return 0, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", "curl/7.79.1")
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

// 生成css标记
func cssA(href string, color, bgColor string) string {
	return fmt.Sprintf(`a[href="%s"] { padding: 3px; color: %s; background-color: %s; }`, href, color, bgColor)
}
