
# $release_dir="bin"

# If (!(test-path $release_dir)){
#     md $release_dir
# }
 
# go env -w GOOS="linux"
# go build -trimpath -o  route ./cmd

go env -w GOOS="windows"
go build -trimpath -o  route.exe ./cmd
