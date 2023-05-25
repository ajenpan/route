
$release_dir="bin"

If (!(test-path $release_dir)){
    md $release_dir
}
 
# build for linux
go env -w GOOS="linux"
go build -o $release_dir/route ./cmd

go env -w GOOS="windows"
go build -o $release_dir/route.exe ./cmd
