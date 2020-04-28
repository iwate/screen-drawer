package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/therecipe/qt/core"
	"github.com/therecipe/qt/gui"
	"github.com/therecipe/qt/widgets"
)

const sdPenWidth = 10
const sdPenAlpha = 128

// SDPoint is screen drawer point struct
type SDPoint struct {
	X float64
	Y float64
}

// SDStroke is screen drawer stroke struct
type SDStroke struct {
	ID     int64
	R      int
	G      int
	B      int
	Points []SDPoint
	C      chan int
}

// SDMessage is draw request
type SDMessage struct {
	ID int64   `json:"id"`
	R  int     `json:"r"`
	G  int     `json:"g"`
	B  int     `json:"b"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

func (s *SDStroke) compare(id int64, r int, g int, b int) bool {
	return s.ID == id && s.R == r && s.G == g && s.B == b
}

func (s *SDStroke) compareStroke(o *SDStroke) bool {
	return s.compare(o.ID, o.R, o.G, o.B)
}

func (s *SDStroke) compareMessage(o *SDMessage) bool {
	return s.compare(o.ID, o.R, o.G, o.B)
}

var messages = make(chan SDMessage)

var origin = "https://localhost:44190"

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// オリジンチェック意図的に外してみる
		// Zoom問題みたいにあらゆるサイトからアクセスできてしまうが
		// カメラやマイクがつながっているわけではないため危険度は下～中くらい
		// 最悪、画面を塗りたくられる攻撃をされるが何かが盗まれることはないはず
		//return r.Header.Get("Origin") == origin
		return true
	},
}

var mutex = &sync.Mutex{}

var strokes = []SDStroke{}

func getIndexOfStrokes(id int64, r int, g int, b int) int {
	for i, s := range strokes {
		if s.compare(id, r, g, b) {
			return i
		}
	}
	return -1
}

func getIndexOfStrokesByStroke(o *SDStroke) int {
	return getIndexOfStrokes(o.ID, o.R, o.G, o.B)
}

func getIndexOfStrokesByMessage(o *SDMessage) int {
	return getIndexOfStrokes(o.ID, o.R, o.G, o.B)
}

func removeStroke(index int) {
	ss := append(strokes[:index], strokes[index+1:]...)
	strokes = make([]SDStroke, len(ss))
	copy(strokes, ss)
}

func appendStroke(message *SDMessage) *SDStroke {
	points := make([]SDPoint, 1, 100)
	points[0] = SDPoint{X: message.X, Y: message.Y}
	stroke := SDStroke{
		ID:     message.ID,
		R:      message.R,
		G:      message.G,
		B:      message.B,
		Points: points,
		C:      make(chan int),
	}
	strokes = append(strokes, stroke)
	return &stroke
}

func appendPoint(index int, x float64, y float64) {
	strokes[index].Points = append(strokes[index].Points, SDPoint{X: x, Y: y})
}

func debounceRemoveStroke(stroke *SDStroke, done chan int) {
	duration := time.Second * 10
	timer := time.NewTimer(duration)

	for {
		select {
		case <-stroke.C:
			timer.Stop()
			timer.Reset(duration)
		case <-timer.C:
			mutex.Lock()
			index := getIndexOfStrokesByStroke(stroke)
			removeStroke(index)
			mutex.Unlock()
			done <- 1
			break
		}
	}
}

func handleMessages(window *widgets.QMainWindow) {
	update := make(chan int)
	go func() {
		for {
			<-update
			window.Update()
		}
	}()
	for {

		message := <-messages

		mutex.Lock()

		index := getIndexOfStrokesByMessage(&message)
		if index != -1 {
			appendPoint(index, message.X, message.Y)
			strokes[index].C <- 1
		} else {
			stroke := appendStroke(&message)
			go debounceRemoveStroke(stroke, update)
		}

		mutex.Unlock()

		update <- 1
	}
}

func exists(name string) bool {
	_, err := os.Stat(name)
	return !os.IsNotExist(err)
}

var home, _ = os.UserHomeDir()
var cert = filepath.Join(home, "sd_cert.cer")
var key = filepath.Join(home, "sd_key.pem")

func main() {
	if !exists(cert) || !exists(key) {
		generateCert()
	}
	if len(os.Args) > 2 {
		origin = os.Args[1]
	} else if o := os.Getenv("VO_ORIGIN"); o != "" {
		origin = o
	}
	app := widgets.NewQApplication(len(os.Args), os.Args)
	screen := gui.QGuiApplication_PrimaryScreen().Geometry()
	w := screen.Width()
	h := screen.Height()
	wf := float64(w)
	hf := float64(h)
	window := widgets.NewQMainWindow(nil, core.Qt__WindowStaysOnTopHint|core.Qt__FramelessWindowHint|core.Qt__WindowTransparentForInput)
	window.SetMinimumSize2(w, h)
	window.SetAttribute(core.Qt__WA_TranslucentBackground, true)
	window.SetStyleSheet("background:transparent;")

	window.ConnectPaintEvent(func(event *gui.QPaintEvent) {
		painter := gui.NewQPainter2(window)
		painter.SetCompositionMode((gui.QPainter__CompositionMode_Clear))
		painter.EraseRect3(window.Geometry())
		painter.SetCompositionMode((gui.QPainter__CompositionMode_DestinationAtop))

		for _, stroke := range strokes {
			color := gui.NewQColor3(stroke.R, stroke.G, stroke.B, sdPenAlpha)
			brush := gui.NewQBrush3(color, core.Qt__SolidPattern)
			painter.SetPen(gui.NewQPen4(brush, sdPenWidth, core.Qt__SolidLine, core.Qt__RoundCap, core.Qt__RoundJoin))

			pointsLen := len(stroke.Points)
			if pointsLen == 0 {
				continue
			}

			if pointsLen == 1 {
				p := stroke.Points[0]
				painter.DrawPoint3(int(p.X*wf), int(p.Y*hf))
				continue
			}

			for i := 1; i < pointsLen; i++ {
				p1 := stroke.Points[i-1]
				p2 := stroke.Points[i]
				painter.DrawLine3(int(p1.X*wf), int(p1.Y*hf), int(p2.X*wf), int(p2.Y*hf))
			}
		}
	})

	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, "index.html")
		})

		http.HandleFunc("/draw", func(w http.ResponseWriter, r *http.Request) {
			go handleMessages(window)
			websocket, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				log.Fatal("error upgrading GET request to a websocket::", err)
			}
			defer websocket.Close()

			for {
				var message SDMessage
				err := websocket.ReadJSON(&message)
				if err != nil {
					log.Printf("error occurred while reading message: %v", err)
					break
				}
				messages <- message
			}
		})

		err := http.ListenAndServeTLS(":44190", cert, key, nil)
		if err != nil {
			log.Fatal("error starting http server::", err)
			return
		}
	}()

	window.Show()
	app.Exec()
}

func generateCert() {

	priv, err := rsa.GenerateKey(rand.Reader, 2048)

	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	notBefore := time.Now()

	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := []string{"localhost"}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}

	certOut, err := os.Create(cert)
	if err != nil {
		log.Fatalf("Failed to open cert.cer for writing: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		log.Fatalf("Failed to write data to cert.cer: %v", err)
	}
	if err := certOut.Close(); err != nil {
		log.Fatalf("Error closing cert.cer: %v", err)
	}
	log.Print("wrote cert.cer\n")

	keyOut, err := os.OpenFile(key, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Failed to open key.pem for writing: %v", err)
		return
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Fatalf("Unable to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		log.Fatalf("Failed to write data to key.pem: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		log.Fatalf("Error closing key.pem: %v", err)
	}
	log.Print("wrote key.pem\n")
}
