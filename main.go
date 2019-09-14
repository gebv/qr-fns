package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

var (
	apiLogin = os.Getenv("CHECKFNS_APILOGIN")
	apiPwd   = os.Getenv("CHECKFNS_APIPWD")
	debug    = os.Getenv("CHECKFNS_DEBUG")
)

// Данные из QR
// t=20190418T211655&s=3943.26&fn=9282000100072197&i=64318&fp=2918241905&n=1
//
// 1. Проверка чека, в случае успеха возвращается 204 код
// GET https://proverkacheka.nalog.ru:9999/v1/ofds/*/inns/*/fss/9282000100072197/operations/1/tickets/64318?fiscalSign=2918241905&date=2019-04-18T21:16:55&sum=394326 HTTP/1.1
//
// 2. Получение данных
// GET https://proverkacheka.nalog.ru:9999/v1/inns/*/kkts/*/fss/9282000100072197/tickets/64318?fiscalSign=2918241905&sendToEmail=no

func main() {
	flag.Parse()

	qrRaw := flag.Arg(0)
	if isDebug() {
		log.Println("[DEBUG] input data:", qrRaw)
	}
	if qrRaw == "" {
		log.Println("invalid arguments: empty input data")
		os.Exit(1)
		return
	}
	qr := parseQRData(qrRaw)
	if qr == nil {
		log.Printf("invalid qr code: %q\n", qrRaw)
		os.Exit(1)
		return
	}
	if !existsCheck(qr) {
		os.Exit(1)
		return
	}
	log.Println("QR args 👌")
	dat, ok := getData(qr)
	if !ok {
		os.Exit(1)
		return
	}
	fmt.Fprintln(os.Stdout, string(dat))
}

type QRPayload struct {
	TimeRaw string
	//
	SumRaw string

	// FN Номер ФН (Фискальный Номер) — 16-значный номер.
	FN string

	// I Номер ФД (Фискальный документ) — до 10 знаков.
	I string

	// FP Номер ФПД (Фискальный Признак Документа, также известный как ФП) — до 10 знаков.
	FP string

	// N Вид кассового чека. В чеке помечается как n=1 (приход) и n=2 (возврат прихода).
	N string
}

// Возвращает истину если все поля валидные (не пустые).
func (p QRPayload) Valid() bool {
	if p.TimeRaw == "" {
		return false
	}

	if p.SumRaw == "" {
		return false
	}

	if p.FN == "" {
		return false
	}

	if p.I == "" {
		return false
	}

	if p.FP == "" {
		return false
	}

	if p.N == "" {
		return false
	}

	return true
}

// FormatDateTime Возвращает в формате 2019-04-18T21:16:55
func (p QRPayload) FormatDateTime() string {
	if len(p.TimeRaw) == 15 {
		return p.TimeRaw[0:4] + "-" + p.TimeRaw[4:6] + "-" + p.TimeRaw[6:8] + "T" +
			p.TimeRaw[9:11] + ":" + p.TimeRaw[11:13] + ":" + p.TimeRaw[13:15]
	}

	if len(p.TimeRaw) == 13 {
		// без секунд
		return p.TimeRaw[0:4] + "-" + p.TimeRaw[4:6] + "-" + p.TimeRaw[6:8] + "T" +
			p.TimeRaw[9:11] + ":" + p.TimeRaw[11:13]
	}
	panic("invalid format datetime from QR")
}

// FormatSum возвращает сумму в копейках
func (p QRPayload) FormatSum() string {
	return strings.Replace(p.SumRaw, ".", "", -1)
}

func parseQRData(dat string) *QRPayload {
	qrArgs, err := url.ParseQuery(dat)
	if err != nil {
		return nil
	}
	return &QRPayload{
		TimeRaw: qrArgs.Get("t"),
		SumRaw:  qrArgs.Get("s"),
		FN:      qrArgs.Get("fn"),
		I:       qrArgs.Get("i"),
		FP:      qrArgs.Get("fp"),
		N:       qrArgs.Get("n"),
	}
}

// возвращает истину если чек валидный и существует
func existsCheck(qr *QRPayload) bool {
	urlFormat := "https://proverkacheka.nalog.ru:9999/v1/ofds/*/inns/*/fss/%s/operations/%s/tickets/%s?fiscalSign=%s&date=%s&sum=%s"
	urlRaw := fmt.Sprintf(urlFormat,
		qr.FN,
		qr.N,
		qr.I,
		qr.FP,
		qr.FormatDateTime(),
		qr.FormatSum(),
	)
	url, err := url.Parse(urlRaw)
	if err != nil {
		log.Printf("failed check: invalid url %q err=%q\n", urlRaw, err)
		return false
	}

	req, _ := http.NewRequest("GET", url.String(), nil)
	req.SetBasicAuth(apiLogin, apiPwd)
	req.Header.Set("Device-OS", "Android 4.4.4")
	req.Header.Set("Device-Id", randString(32))

	if isDebug() {
		dat, _ := httputil.DumpRequest(req, true)
		log.Println("[DEBUG] raw request:")
		log.Println(string(dat))
	}

	res, err := http.DefaultClient.Do(req)

	if isDebug() {
		dat, _ := httputil.DumpResponse(res, true)
		log.Println("[DEBUG] raw response:")
		log.Println(string(dat))
	}

	if err != nil {
		log.Printf("failed check: err=%q\n", err)
		return false
	}
	if res.StatusCode != 204 {
		log.Printf("failed check: status code not 204\n")
		return false
	}

	return true
}

// возвращает содержимое чека если он существует
func getData(qr *QRPayload) ([]byte, bool) {
	urlFormat := "https://proverkacheka.nalog.ru:9999/v1/inns/*/kkts/*/fss/%s/tickets/%s?fiscalSign=%s&sendToEmail=no"
	urlRaw := fmt.Sprintf(urlFormat,
		qr.FN,
		qr.I,
		qr.FP,
	)

	url, err := url.Parse(urlRaw)
	if err != nil {
		log.Printf("failed get data: invalid url %q err=%q\n", urlRaw, err)
		return nil, false
	}

	req, _ := http.NewRequest("GET", url.String(), nil)
	req.SetBasicAuth(apiLogin, apiPwd)
	req.Header.Set("Device-OS", "Android 4.4.4")
	req.Header.Set("Device-Id", randString(32))

	if isDebug() {
		dat, _ := httputil.DumpRequest(req, true)
		log.Println("[DEBUG] raw request:")
		log.Println(string(dat))
	}

	res, err := http.DefaultClient.Do(req)

	if isDebug() {
		dat, _ := httputil.DumpResponse(res, true)
		log.Println("[DEBUG] raw response:")
		log.Println(string(dat))
	}

	if err != nil {
		log.Printf("failed get data: err=%q\n", err)
		return nil, false
	}
	if res.StatusCode != 200 {
		log.Printf("failed get data: status code not 200\n")
		return nil, false
	}

	dat, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Printf("failed get data: error read body err%s\n", err)
		return nil, false
	}

	return dat, true
}

func randString(len int) string {
	b := make([]byte, len)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func isDebug() bool {
	return debug != ""
}

///
