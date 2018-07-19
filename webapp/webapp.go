// go run webapp.go -a 0.0.0.0 -p 8080 -r .

package main

import (
    "bytes"
    "encoding/base64"
    "flag"
    "fmt"
    "html/template"
    "image"
    "image/gif"
    "image/jpeg"
    "image/png"
    "io"
    "io/ioutil"
    "log"
    "net"
    "net/http"
    "os"
    "os/exec"
    "path"
    "regexp"
    "runtime"
    "sort"
    "strconv"
    "strings"
)

var addr = flag.String("a", "0.0.0.0", "server host address")
var port = flag.Int("p", 8080, "server port number")
var root = flag.String("r", ".", "file system path to browse from")
var cmdBufLen = 1024
var browserAddr = "127.0.0.1"
var gopath = os.Getenv("GOPATH")
var rootmarker = ".webapp"
var srcpath string

// default params for localhost testing
var cliIaDef = "1-ff00:0:111"
var serIaDef = "1-ff00:0:112"
var cliPortDef = "30001"
var serPortDefBwt = "30100"
var serPortDefImg = "42002"
var serPortDefSen = "42003"
var serDefAddr = "127.0.0.2"

var imgTemplate = `<!doctype html><html lang="en"><head></head><body>
<a href="{{.ImgUrl}}" target="_blank"><img src="data:image/jpg;base64,{{.JpegB64}}">
</a></body>`

var regexImageFiles = `([^\s]+(\.(?i)(jp?g|png|gif))$)`

var cfgFileCliUser = "config/clients_user.json"
var cfgFileSerUser = "config/servers_user.json"
var cfgFileCliDef = "config/clients_default.json"
var cfgFileSerDef = "config/servers_default.json"

func main() {
    flag.Parse()
    _, srcfile, _, _ := runtime.Caller(0)
    srcpath = path.Dir(srcfile)

    // generate client/server default
    genClientNodeDefaults(path.Join(srcpath, cfgFileCliDef))
    genServerNodeDefaults(path.Join(srcpath, cfgFileSerUser))
    refreshRootDirectory()
    appsBuildCheck("bwtester")
    appsBuildCheck("camerapp")
    appsBuildCheck("sensorapp")

    http.HandleFunc("/", mainHandler)
    fsStatic := http.FileServer(http.Dir(path.Join(srcpath, "static")))
    http.Handle("/static/", http.StripPrefix("/static/", fsStatic))
    fsImageFetcher := http.FileServer(http.Dir('.'))
    http.Handle("/images/", http.StripPrefix("/images/", fsImageFetcher))
    fsFileBrowser := http.FileServer(http.Dir(*root))
    http.Handle("/files/", http.StripPrefix("/files/", fsFileBrowser))

    http.HandleFunc("/command", commandHandler)
    http.HandleFunc("/imglast", findImageHandler)
    http.HandleFunc("/txtlast", findImageInfoHandler)
    http.HandleFunc("/getnodes", getNodesHandler)

    log.Printf("Browser access at http://%s:%d.\n", browserAddr, *port)
    log.Printf("File browser root: %s\n", *root)
    log.Printf("Listening on %s:%d...\n", *addr, *port)
    log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", *addr, *port), nil))
}

// Reads locally generated file for this IA's name, if written
func getLocalIa() string {
    scionroot := "src/github.com/scionproto/scion"
    filepath := path.Join(gopath, scionroot, "gen/ia")
    b, err := ioutil.ReadFile(filepath)
    if err != nil {
        log.Println("ioutil.ReadFile() error: " + err.Error())
        return cliIaDef
    } else {
        return string(b)
    }
}

// Makes interfaces sortable, by preferred name
type byPrefInterface []net.Interface

func isInterfaceEnp(c net.Interface) bool {
    return strings.HasPrefix(c.Name, "enp")
}

func (c byPrefInterface) Len() int {
    return len(c)
}

func (c byPrefInterface) Swap(i, j int) {
    c[i], c[j] = c[j], c[i]
}

func (c byPrefInterface) Less(i, j int) bool {
    // sort "enp" interfaces first, then alphabetically
    if isInterfaceEnp(c[i]) && !isInterfaceEnp(c[j]) {
        return true
    }
    if !isInterfaceEnp(c[i]) && isInterfaceEnp(c[j]) {
        return false
    }
    return c[i].Name < c[j].Name
}

// Creates server defaults for localhost testing
func genServerNodeDefaults(ser_fp string) {
    jsonBlob := []byte(`{ `)
    json := []byte(`"bwtester": [{"name":"localhost","isdas":"` +
        serIaDef + `", "addr":"` + serDefAddr + `","port":` + serPortDefBwt + `}], `)
    jsonBlob = append(jsonBlob, json...)
    json = []byte(`"camerapp": [{"name":"localhost","isdas":"` +
        serIaDef + `", "addr":"` + serDefAddr + `","port":` + serPortDefImg + `}], `)
    jsonBlob = append(jsonBlob, json...)
    json = []byte(`"sensorapp": [{"name":"localhost","isdas":"` +
        serIaDef + `", "addr":"` + serDefAddr + `","port":` + serPortDefSen + `}] `)
    jsonBlob = append(jsonBlob, json...)
    jsonBlob = append(jsonBlob, []byte(` }`)...)
    err := ioutil.WriteFile(ser_fp, jsonBlob, 0644)
    if err != nil {
        log.Println("ioutil.WriteFile() error: " + err.Error())
    }
}

// Queries network interfaces and writes local client SCION addresses as json
func genClientNodeDefaults(cli_fp string) {
    cisdas := getLocalIa()
    cport := cliPortDef

    // find interface addresses
    jsonBlob := []byte(`{ "all": [ `)
    ifaces, err := net.Interfaces()
    if err != nil {
        log.Println("net.Interfaces() error: " + err.Error())
        return
    }
    sort.Sort(byPrefInterface(ifaces))
    idx := 0
    for _, i := range ifaces {
        addrs, err := i.Addrs()
        if err != nil {
            log.Println("i.Addrs() error: " + err.Error())
            continue
        }
        for _, a := range addrs {
            if ipnet, ok := a.(*net.IPNet); ok {
                if ipnet.IP.To4() != nil {
                    if idx > 0 {
                        jsonBlob = append(jsonBlob, []byte(`, `)...)
                    }
                    cname := i.Name
                    caddr := ipnet.IP.String()
                    jsonInterface := []byte(`{"name":"` + cname + `", "isdas":"` +
                        cisdas + `", "addr":"` + caddr + `","port":` + cport + `}`)
                    jsonBlob = append(jsonBlob, jsonInterface...)
                    idx++
                }
            }
        }
    }
    jsonBlob = append(jsonBlob, []byte(` ] }`)...)
    err = ioutil.WriteFile(cli_fp, jsonBlob, 0644)
    if err != nil {
        log.Println("ioutil.WriteFile() error: " + err.Error())
    }
}

// Handles loading index.html for user at root.
func mainHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    w.WriteHeader(http.StatusOK)

    filepath := path.Join(srcpath, "template/index.html")
    data, err := ioutil.ReadFile(filepath)
    if err != nil {
        log.Fatal("ioutil.ReadFile() error: " + err.Error())
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }
    w.Header().Set("Content-Length", fmt.Sprint(len(data)))
    fmt.Fprint(w, string(data))
}

// Handles parsing SCION addresses to execute client app and write results.
func commandHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    iaSer := r.PostFormValue("ia_ser")
    iaCli := r.PostFormValue("ia_cli")
    addrSer := r.PostFormValue("addr_ser")
    addrCli := r.PostFormValue("addr_cli")
    portSer := r.PostFormValue("port_ser")
    portCli := r.PostFormValue("port_cli")
    addlOpt := r.PostFormValue("addl_opt")
    bwCS := r.PostFormValue("bw_cs")
    bwSC := r.PostFormValue("bw_sc")
    appSel := r.PostFormValue("apps")

    // execute scion go client app with client/server commands
    binname := getClientLocationBin(appSel)
    if binname == "" {
        fmt.Fprintf(w, "Unknown SCION client app. Is one selected?")
        return
    }
    optClient := fmt.Sprintf("-c=%s", fmt.Sprintf("%s,[%s]:%s", iaCli, addrCli, portCli))
    optServer := fmt.Sprintf("-s=%s", fmt.Sprintf("%s,[%s]:%s", iaSer, addrSer, portSer))
    log.Printf("Executing: %s %s %s %s %s %s\n", binname, optClient, optServer, bwCS, bwSC, addlOpt)
    cmd := exec.Command(binname, optServer, optClient, bwCS, bwSC, addlOpt)

    // pipe command results to page
    pipeReader, pipeWriter := io.Pipe()
    cmd.Stdout = pipeWriter
    cmd.Stderr = pipeWriter
    go writeCmdOutput(w, pipeReader)
    cmd.Run()
    pipeWriter.Close()
}

func appsBuildCheck(app string) {
    binname := getClientLocationBin(app)
    installpath := path.Join(gopath, "bin", binname)
    // check for install, and install only if needed
    if _, err := os.Stat(installpath); os.IsNotExist(err) {
        filepath := getClientLocationSrc(app)
        cmd := exec.Command("go", "install")
        cmd.Dir = path.Dir(filepath)
        log.Printf("Installing %s...\n", app)
        cmd.Run()
    } else {
        log.Printf("Existing install, found %s...\n", app)
    }
}

// Parses html selection and returns name of app binary.
func getClientLocationBin(app string) string {
    var binname string
    switch app {
    case "sensorapp":
        binname = "sensorfetcher"
    case "camerapp":
        binname = "imagefetcher"
    case "bwtester":
        binname = "bwtestclient"
    }
    return binname
}

// Parses html selection and returns location of app source.
func getClientLocationSrc(app string) string {
    slroot := "src/github.com/perrig/scionlab"
    var filepath string
    switch app {
    case "sensorapp":
        filepath = path.Join(gopath, slroot, "sensorapp/sensorfetcher/sensorfetcher.go")
    case "camerapp":
        filepath = path.Join(gopath, slroot, "camerapp/imagefetcher/imagefetcher.go")
    case "bwtester":
        filepath = path.Join(gopath, slroot, "bwtester/bwtestclient/bwtestclient.go")
    }
    return filepath
}

// Handles piping command line output to http response writer.
func writeCmdOutput(w http.ResponseWriter, pr *io.PipeReader) {
    buf := make([]byte, cmdBufLen)
    for {
        n, err := pr.Read(buf)
        if err != nil {
            pr.Close()
            break
        }
        output := buf[0:n]
        w.Write(output)
        if f, ok := w.(http.Flusher); ok {
            f.Flush()
        }
        for i := 0; i < n; i++ {
            buf[i] = 0
        }
    }
}

// Handles writing jpeg image to http response writer by content-type.
func writeJpegContentType(w http.ResponseWriter, img *image.Image) {
    buf := new(bytes.Buffer)
    err := jpeg.Encode(buf, *img, nil)
    if err != nil {
        log.Println("jpeg.Encode() error: " + err.Error())
    }
    w.Header().Set("Content-Type", "image/jpeg")
    w.Header().Set("Content-Length", strconv.Itoa(len(buf.Bytes())))
    _, werr := w.Write(buf.Bytes())
    if werr != nil {
        log.Println("w.Write() image error: " + werr.Error())
    }
}

// Handles writing jpeg image to http response writer by image template.
func writeJpegTemplate(w http.ResponseWriter, img *image.Image, fname string) {
    buf := new(bytes.Buffer)
    err := jpeg.Encode(buf, *img, nil)
    if err != nil {
        log.Println("jpeg.Encode() error: " + err.Error())
    }
    str := base64.StdEncoding.EncodeToString(buf.Bytes())
    tmpl, err := template.New("image").Parse(imgTemplate)
    if err != nil {
        log.Println("tmpl.Parse() image error: " + err.Error())
    } else {
        url := fmt.Sprintf("http://%s:%d/%s/%s", browserAddr, *port, "images", fname)
        data := map[string]interface{}{"JpegB64": str, "ImgUrl": url}
        err := tmpl.Execute(w, data)
        if err != nil {
            log.Println("tmpl.Execute() image error: " + err.Error())
        }
    }
}

// Helper method to find most recently modified regex extension filename in dir.
func findNewestFileExt(dir, extRegEx string) (imgFilename string, imgTimestamp int64) {
    files, _ := ioutil.ReadDir(dir)
    for _, f := range files {
        fi, err := os.Stat(path.Join(dir, f.Name()))
        if err != nil {
            log.Println("os.Stat() error: " + err.Error())
        }
        matched, err := regexp.MatchString(extRegEx, f.Name())
        if matched {
            modTime := fi.ModTime().Unix()
            if modTime > imgTimestamp {
                imgTimestamp = modTime
                imgFilename = f.Name()
            }
        }
    }
    return
}

// Handles locating most recent image and writing text info data about it.
func findImageInfoHandler(w http.ResponseWriter, r *http.Request) {
    filesDir := "."
    imgFilename, _ := findNewestFileExt(filesDir, regexImageFiles)
    if imgFilename == "" {
        return
    }
    fileText := imgFilename
    fmt.Fprintf(w, fileText)
}

// Handles locating most recent image formatting it for graphic display in response.
func findImageHandler(w http.ResponseWriter, r *http.Request) {
    filesDir := "."
    imgFilename, _ := findNewestFileExt(filesDir, regexImageFiles)
    if imgFilename == "" {
        fmt.Fprint(w, "Error: Unable to find image file locally.")
        return
    }
    log.Println("Found image file: " + imgFilename)
    imgFile, err := os.Open(path.Join(filesDir, imgFilename))
    if err != nil {
        log.Println("os.Open() error: " + err.Error())
    }
    defer imgFile.Close()
    _, imageType, err := image.Decode(imgFile)
    if err != nil {
        log.Println("image.Decode() error: " + err.Error())
    }
    log.Println("Found image type: " + imageType)
    // reset file pointer to beginning
    imgFile.Seek(0, 0)
    var rawImage image.Image
    switch imageType {
    case "gif":
        rawImage, err = gif.Decode(imgFile)
    case "png":
        rawImage, err = png.Decode(imgFile)
    case "jpeg":
        rawImage, err = jpeg.Decode(imgFile)
    default:
        panic("Unhandled image type!")
    }
    if err != nil {
        log.Println("png.Decode() error: " + err.Error())
    }
    writeJpegTemplate(w, &rawImage, imgFilename)
}

func getNodesHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    nodes := r.PostFormValue("node_type")
    var fp string
    switch nodes {
    case "clients_default":
        fp = path.Join(srcpath, cfgFileCliDef)
    case "servers_default":
        fp = path.Join(srcpath, cfgFileSerDef)
    case "clients_user":
        fp = path.Join(srcpath, cfgFileCliUser)
    case "servers_user":
        fp = path.Join(srcpath, cfgFileSerUser)
    default:
        panic("Unhandled nodes type!")
    }
    raw, err := ioutil.ReadFile(fp)
    if err != nil {
        log.Println("ioutil.ReadFile() error: " + err.Error())
    }
    fmt.Fprintf(w, string(raw))
}

// Used to workaround cache-control issues by ensuring root specified by user
// has updated last modified date by writing a .webapp file
func refreshRootDirectory() {
    cli_fp := path.Join(srcpath, *root, rootmarker)
    err := ioutil.WriteFile(cli_fp, []byte(``), 0644)
    if err != nil {
        log.Println("ioutil.WriteFile() error: " + err.Error())
    }
}

type FileBrowseResponseWriter struct {
    http.ResponseWriter
}

// Prevents caching directory listings based on directory last modified date.
// This is especailly a problem in Chrome, and can serve the browser stale listings.
func (w FileBrowseResponseWriter) WriteHeader(code int) {
    if code == 200 {
        w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate, proxy-revalidate")
    }
    w.ResponseWriter.WriteHeader(code)
}

// Handles custom filtering of file browsing content
func fileBrowseHandler(h http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rw := FileBrowseResponseWriter{w}
        h.ServeHTTP(rw, r)
    })
}
