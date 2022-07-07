#!/usr/bin/env bash

package_path=$1
if [[ -z "$package_path" ]]; then
  echo "usage: $0 <package-path> $1 <package-name>" 
  exit 1
fi

package=$2

platforms=(
    "darwin/amd64"
    "darwin/arm64"
    "linux/386"
    "linux/amd64"
    "linux/arm"
    "linux/arm64"
    "windows/386"
    "windows/amd64"
    "windows/arm"
)

rm -rf dist/$package/*
for platform in "${platforms[@]}"
do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}``
    GOARCH=${platform_split[1]}
    echo 'Building' $GOOS-$GOARCH
    output_name=toxiproxy-$package-$GOOS-$GOARCH

    env GOOS=$GOOS GOARCH=$GOARCH go build -v -o dist/$package/$output_name $package_path > /dev/null

    if [ $? -ne 0 ]; then
        echo 'An error has occurred! Aborting the script execution...'
        exit 1
    fi

    cd dist/$package
    mv $output_name toxiproxy-$package
    tar -czvf $output_name.tar.gz toxiproxy-$package
    rm -rf toxiproxy-$package
    cd ../..
done