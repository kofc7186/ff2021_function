package ff2021function

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/iterator"

	"github.com/skip2/go-qrcode"
)

var projectID = "fishfry2021"
var topicID = "print_queue"
var bucketID = "ff2021"

var pubsubClient *pubsub.Client
var storageClient *storage.Client
var driveService *drive.Service

func init() {
	var err error
	storageClient, err = storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("storage.NewClient: %v", err)
	}
	log.Print("using GCP Project ID ", projectID)
	pubsubClient, err = pubsub.NewClient(context.Background(), projectID)
	if err != nil {
		log.Fatalf("pubsub.NewClient: %v", err)
	}
	driveService, err = drive.NewService(context.Background())
	if err != nil {
		log.Fatalf("error initializing drive service: %v", err)
	}
}

//MakeDocAndPrint makes the document using HTML template, writes to drive/GCS and prints
func MakeDocAndPrint(w http.ResponseWriter, r *http.Request) {
	son := r.URL.Query().Get("son")
	skipPrint := r.URL.Query().Get("skipPrint") == "true"
	folderId := r.URL.Query().Get("folderId")
	// makeDoc and get as []byte
	data, err := makeDoc(r.Body)
	if err != nil {
		log.Println(fmt.Errorf("error creating doc: %w", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	name := son + "-" + fmt.Sprint(time.Now().Unix()) + ".pdf"

	if skipPrint {
		//write to drive
		file := drive.File{Name: name}
		if folderId != "" {
			file.Parents = []string{folderId}
		}
		df, err := driveService.Files.Create(&file).Media(bytes.NewReader(data)).Do()
		if err != nil {
			log.Println(fmt.Errorf("error writing PDF to drive: %w", err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		//wg.Wait()
		driveService.Files.Update(df.Id, df).AddParents(folderId).Do()

		json, _ := json.Marshal(df)

		fmt.Fprint(w, string(json))
		//fmt.Fprintf(w, "{\"son\":%v}", son)
	} else {
		//var wg sync.WaitGroup
		//wg.Add(1)
		// goroutine to save to GCS in background
		//go func(ctx context.Context, name string, data []byte) {
		bkt := storageClient.Bucket(bucketID)

		objWriter := bkt.Object(name).NewWriter(context.Background())
		defer objWriter.Close()

		log.Printf("writing %v to GCS", name)
		if _, err := objWriter.Write(data); err != nil {
			log.Printf("error writing %v to GCS: %v", name, err)
		}
		//wg.Done()
		//}(r.Context(), name, data)

		// call printPDF
		id, err := printPDF(r.Context(), son, "", data)
		if err != nil {
			log.Println(fmt.Errorf("error publishing msg: %w", err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "{\"messageID\":%v}", id)
		//wg.Wait()
	}
}

//AppSheetWebhook contains entire object from AppSheet
type AppSheetWebhook struct {
	UpdateMode, Application, TableName, UserName, At string
	Data                                             AppSheetData
}

//AppSheetData contains data from AppSheet
type AppSheetData struct {
	SquareOrderNumber string `json:"Square Order Number"`
	CustomerName      string `json:"Customer Name"`
	CustomerPhone     string `json:"Customer Phone"`
	Image             string
	CarColor          string `json:"Car Color"`
	CarMakeModel      string `json:"Car Make/Model"`
	CarNumber         string `json:"Car Number"`
	AdultMeals        string `json:"Adult Meals"`
	KidsMeals         string `json:"Kids Meals"`
	Total             string
	TimeSlot          string `json:"Time Slot"`
	OrderStatus       string `json:"Order Status"`
	ProcessedTime     string `json:"Processed Time"`
	ArrivalTime       string `json:"Arrival Time"`
	CarDescription    string `json:"Car Description"`
	BeerOrder         string `json:"Beer Order"`
	Note              string `json:"Note"`
	Sauces            uint
	Desserts          uint
	Doubles           uint
	Singles           uint
	Beers             map[string]uint
	QRCode            string
}

func makeDoc(body io.ReadCloser) ([]byte, error) {
	input, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	var asw AppSheetWebhook
	if err := json.Unmarshal(input, &asw); err != nil {
		return nil, err
	}
	formUrl := "https://docs.google.com/forms/d/e/1FAIpQLSfEeu-b86rljSRoCPIEcdrrn-Y3HLebCP4vKcAcSsyXp0WRLw/formResponse?usp=pp_url&entry.1999895754=" + asw.Data.SquareOrderNumber + "&submit=Submit"
	png, err := qrcode.Encode(formUrl, qrcode.Medium, 256)
	if err != nil {
		return nil, err
	}
	asw.Data.QRCode = base64.StdEncoding.EncodeToString(png)
	if asw.Data.CarColor == "unknown" {
		if asw.Data.CarMakeModel == "unknown unknown" {
			asw.Data.CarColor = ""
			asw.Data.CarMakeModel = ""
		} else {
			asw.Data.CarColor = " "
			asw.Data.CarMakeModel = strings.TrimSpace(strings.Replace(asw.Data.CarMakeModel, "unknown", "", 1))
		}
	}
	doublesUInt, _ := strconv.ParseUint(asw.Data.AdultMeals, 10, 0)
	asw.Data.Doubles = uint(doublesUInt / 2)
	asw.Data.Singles = uint(doublesUInt) % 2
	asw.Data.Sauces = asw.Data.Doubles*2 + asw.Data.Singles
	kidsUInt, _ := strconv.ParseUint(asw.Data.KidsMeals, 10, 0)
	asw.Data.Desserts = uint(kidsUInt) + asw.Data.Sauces

	if err := json.Unmarshal([]byte(asw.Data.BeerOrder), &asw.Data.Beers); err != nil {
		return nil, err
	}

	t := template.Must(template.New("template.htm").ParseFiles("./serverless_function_source_code/lib/template.htm"))

	outputHTMLFile, err := ioutil.TempFile(os.TempDir(), asw.Data.SquareOrderNumber+"-*.htm")
	if err != nil {
		return nil, err
	}

	if err := t.Execute(outputHTMLFile, asw.Data); err != nil {
		return nil, err
	}

	outputHTMLFile.Close()
	// //DELETE BELOW
	// htmlData, _ := ioutil.ReadFile(outputHTMLFile.Name())
	// file := drive.File{Name: asw.Data.SquareOrderNumber + ".html"}
	// file.Parents = []string{"1kQhZtltOjRhhvBh-aSTsdYa5d42gJZjk"}
	// df, _ := driveService.Files.Create(&file).Media(bytes.NewReader(htmlData)).Do()
	// driveService.Files.Update(df.Id, df).AddParents("1kQhZtltOjRhhvBh-aSTsdYa5d42gJZjk").Do()
	// //DELETE ABOVE

	fileStat, err := os.Stat(outputHTMLFile.Name())
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Output HTML Size:", fileStat.Size()) // Length in bytes for regular files

	pdfFileName := outputHTMLFile.Name() + ".pdf"
	//shell out to wkhtmltopdf
	cmd := exec.Command("./serverless_function_source_code/wkhtmltopdf", "-q", outputHTMLFile.Name(), pdfFileName)
	cmd.Env = append(cmd.Env, "LD_LIBRARY_PATH=./serverless_function_source_code/lib/")
	b, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("output of wkhtmltopdf: %v", string(b))
		log.Printf("error running wkhtmltopdf: %v\n", err)

		return nil, err
	}
	os.Remove(path.Join(os.TempDir(), outputHTMLFile.Name()))

	return ioutil.ReadFile(pdfFileName)
}

//Print prints document from GCS
func Print(w http.ResponseWriter, r *http.Request) {
	reprint := r.URL.Query().Get("reprint")
	son := r.URL.Query().Get("son")

	data, err := getPDF(r.Context(), son)
	if err != nil {
		log.Println(fmt.Errorf("error fetching pdf: %w", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Print(len(data))

	id, err := printPDF(r.Context(), son, reprint, data)
	if err != nil {
		log.Println(fmt.Errorf("error publishing msg: %w", err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "{\"messageID\":%v}", id)
}

func getPDF(ctx context.Context, son string) ([]byte, error) {
	bkt := storageClient.Bucket(bucketID)

	query := &storage.Query{
		Prefix: son + "-",
	}
	query.SetAttrSelection([]string{"Name"})

	var names []string
	it := bkt.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		names = append(names, attrs.Name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no object found for son %v", son)
	}
	sort.Strings(names)
	name := names[len(names)-1]
	log.Println("name from bucket: ", name)

	obj, err := bkt.Object(name).NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer obj.Close()

	return ioutil.ReadAll(obj)
}

func printPDF(ctx context.Context, son, reprint string, pdfBytes []byte) (string, error) {
	if pdfBytes == nil {
		var err error
		pdfBytes, err = getPDF(ctx, son)
		if err != nil {
			return "", err
		}
	}

	msg := &pubsub.Message{
		Data: pdfBytes,
		Attributes: map[string]string{
			"reprint": reprint,
			"son":     son,
		},
	}

	id, err := pubsubClient.Topic(topicID).Publish(ctx, msg).Get(ctx)
	if err != nil {
		return "", err
	}

	return id, nil
}
