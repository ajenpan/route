
$release_dir="bin"

If (!(test-path $release_dir)){
    md $release_dir
}
 
go env -w GOOS="linux"
go build -trimpath -o $release_dir/route ./cmd

go env -w GOOS="windows"
go build -trimpath -o $release_dir/route.exe ./cmd

# go build -o $release_dir/example-client.exe ./example-client
