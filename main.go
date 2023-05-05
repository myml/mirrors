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
	var sourceURL, mirrorsFile, checkFile string
	flag.StringVar(&sourceURL, "source", "", "源站")
	flag.StringVar(&mirrorsFile, "mirrors", "", "镜像列表文件，包含镜像列表的markdown格式文件")
	flag.StringVar(&checkFile, "checkpoint", "", "检查点文件，比较源站和镜像站该文件是否相同")
	flag.DurationVar(&Timeout, "timeout", Timeout, "request timeout")
	flag.Parse()
	if len(sourceURL) == 0 || len(mirrorsFile) == 0 || len(checkFile) == 0 {
		flag.PrintDefaults()
		return
	}
	links, err := getMarkdownLinks(mirrorsFile)
	if err != nil {
		log.Fatal(err)
	}
	checkMirror(sourceURL, checkFile, links)
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
	sourceLength, err := head(strings.Trim(source, "/") + checkFile)
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
			mirrorLength, err := head(strings.Trim(href, "/") + checkFile)
			if err != nil {
				log.Println(href, "失败", err)
				errStore.Store(href, err.Error())
				return
			}
			if mirrorLength != sourceLength {
				log.Println(href, "失败", "文件过期")
				errStore.Store(href, "文件过期")
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
func head(url string) (ContentLength int64, err error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), Timeout)
	defer cancel()
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", "curl/7.79.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, errors.New(resp.Status)
	}
	return resp.ContentLength, nil
}

// 生成css标记
func cssA(href string, color, bgColor string) string {
	return fmt.Sprintf(`a[href="%s"] { padding: 3px; color: %s; background-color: %s; }`, href, color, bgColor)
}
