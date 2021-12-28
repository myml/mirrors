build-package-zh:
	go run github.com/myml/mirrors -package_url https://myml.dev/developer-center/mirrors/packages_zh.html > packages_zh_mark.css
build-package-en:
	go run github.com/myml/mirrors -package_url https://myml.dev/developer-center/mirrors/packages_en.html > packages_en_mark.css
build-release-en:
	go run github.com/myml/mirrors -release_url https://myml.dev/developer-center/mirrors/releases_en.html > releases_en_mark.css
build-release-zh:
	go run github.com/myml/mirrors -release_url https://myml.dev/developer-center/mirrors/releases_zh.html > releases_zh_mark.css

build-package: build-package-zh build-package-en

build-release: build-release-en build-release-zh

build: build-package-zh build-package-en build-release-en build-release-zh
	
clean:
	rm *.css