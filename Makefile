all : gxuitter

gxuitter : main.go bindata.go
	go build -o gxuitter

bindata.go : data/black.png
	go get github.com/jteeuwen/go-bindata/go-bindata
	go-bindata data

clean :
	-rm bindata.go gxuitter
