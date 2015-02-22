#Ninja Sphere GoLang Limitless Driver

NOT COMPILING AT THIS STAGE

##Building
Run `make` in the directory of the driver

or to develop on mac and run on the sphere
`GOOS=linux GOARCH=arm go build -o limitless main.go driver.go version.go && scp limitless ninja@ninjasphere.local:~/`

##Running
Run `./bin/driver-go-limitless` from the `bin` directory after building 
