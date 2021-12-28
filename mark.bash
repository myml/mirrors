#!/bin/bash -x
for type in packages releases;do
    for lang in zh en;do
        go run github.com/myml/mirrors -${type}_url https://www.deepin.org/developer-center/mirrors/${type}_${lang}.html > ${type}_${lang}_mark.css &
    done
done
wait