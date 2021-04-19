package main

import (

	"fmt"

	"runtime"

)


func main() {

	fmt.Printf("build on $BUILDPLATFORM, build for $TARGETPLATFORM, runtime GOARCH: %s\
", runtime.GOARCH)

}
