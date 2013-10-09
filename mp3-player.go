package main

/*
#cgo pkg-config: gstreamer-1.0
#include <gst/gst.h>


// ******************** 定义消息处理函数 ********************
gboolean bus_call(GstBus *bus, GstMessage *msg, gpointer data)
{
        GMainLoop *loop = (GMainLoop *)data;//这个是主循环的指针，在接受EOS消息时退出循环
        gchar *debug;
        GError *error;

        switch (GST_MESSAGE_TYPE(msg)) {
        case GST_MESSAGE_EOS:
                g_main_loop_quit(loop);
                break;
        case GST_MESSAGE_ERROR:
                gst_message_parse_error(msg,&error,&debug);
                g_free(debug);
                g_printerr("ERROR:%s\n",error->message);
                g_error_free(error);
                g_main_loop_quit(loop);
                break;
        default:
                break;
        }

        return TRUE;
}

static GstBus *pipeline_get_bus(void *pipeline)
{
        return gst_pipeline_get_bus(GST_PIPELINE(pipeline));
}

static void bus_add_watch(void *bus, void *loop)
{
        gst_bus_add_watch(bus, bus_call, loop);
        gst_object_unref(bus);
}

static void set_path(void *play, gchar *path)
{
        g_object_set(G_OBJECT(play), "uri", path, NULL);
}

static void object_unref(void *pipeline)
{
        gst_object_unref(GST_OBJECT(pipeline));
}

static void mp3_ready(void *pipeline)
{
        gst_element_set_state(pipeline, GST_STATE_READY);
}

static void mp3_pause(void *pipeline)
{
        gst_element_set_state(pipeline, GST_STATE_PAUSED);
}

static void mp3_play(void *pipeline)
{
        gst_element_set_state(pipeline, GST_STATE_PLAYING);
}

static void mp3_stop(void *pipeline)
{
        gst_element_set_state(pipeline, GST_STATE_NULL);
}

static void set_mute(void *play)
{
        g_object_set(G_OBJECT(play), "mute", FALSE, NULL);
}

static void set_volume(void *play, int vol)
{
        int ret = vol % 11;

        g_object_set(G_OBJECT(play), "volume", ret/10.0, NULL);
}

*/
import "C"

import (
	"bufio"
	"container/list"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
	"unsafe"
)

const MP3_FILE_MAX = 10

var g_list *list.List
var g_wg *sync.WaitGroup
var g_isOutOfOrder bool
var g_volume_size int = 5

func GString(s string) *C.gchar {
	return (*C.gchar)(C.CString(s))
}

func GFree(s unsafe.Pointer) {
	C.g_free(C.gpointer(s))
}

func walkFunc(fpath string, info os.FileInfo, err error) error {
	if info.IsDir() {
		return nil
	}
	switch filepath.Ext(fpath) {
	case ".mp3", ".wav", ".ogg", ".wma", ".rmvb", ".mov":
	default:
		return nil
	}
	if x, err0 := filepath.Abs(fpath); err != nil {
		err = err0
		return err
	} else {
		p := fmt.Sprintf("file://%s", x)
		g_list.PushBack(p)
	}

	return err
}

func outOfOrder(l *list.List) {
	iTotal := 25
	if iTotal > l.Len() {
		iTotal = l.Len()
	}
	ll := make([]*list.List, iTotal)

	for i := 0; i < iTotal; i++ {
		ll[i] = list.New()
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for e := l.Front(); e != nil; e = e.Next() {
		fpath, ok := e.Value.(string)
		if !ok {
			panic("The path is invalid string")
		}
		if rand.Int()%2 == 0 {
			ll[r.Intn(iTotal)].PushFront(fpath)
		} else {
			ll[r.Intn(iTotal)].PushBack(fpath)
		}
	}

	r0 := rand.New(rand.NewSource(time.Now().UnixNano()))
	l.Init()
	for i := 0; i < iTotal; i++ {
		if r0.Intn(2) == 0 {
			l.PushBackList(ll[i])
		} else {
			l.PushFrontList(ll[i])
		}
		ll[i].Init()
	}
}

func SinglePlayProcess(fpath string, loop *C.GMainLoop) {
	// fmt.Printf("filename[%s]\n", fpath)
	var pipeline *C.GstElement // 定义组件
	var bus *C.GstBus

	switch t := filepath.Ext(fpath); t {
	case ".mp3", ".wav", ".ogg", ".wma", ".rmvb", ".mov":
	default:
		fmt.Printf("不支持此文件格式[%s]\n", t)
		return
	}

	v0 := GString("playbin")
	v1 := GString("play")
	pipeline = C.gst_element_factory_make(v0, v1)
	GFree(unsafe.Pointer(v0))
	GFree(unsafe.Pointer(v1))
	v2 := GString(fpath)
	C.set_path(unsafe.Pointer(pipeline), v2)
	GFree(unsafe.Pointer(v2))

	// 得到 管道的消息总线
	bus = C.pipeline_get_bus(unsafe.Pointer(pipeline))
	if bus == (*C.GstBus)(nil) {
		fmt.Println("GstBus element could not be created.Exiting.")
		return
	}
	C.bus_add_watch(unsafe.Pointer(bus), unsafe.Pointer(loop))

	C.mp3_ready(unsafe.Pointer(pipeline))
	C.mp3_play(unsafe.Pointer(pipeline))

	// 开始循环
	C.g_main_loop_run(loop)
	C.mp3_stop(unsafe.Pointer(pipeline))
	C.object_unref(unsafe.Pointer(pipeline))
}

func PlayProcess(cs chan byte, loop *C.GMainLoop) {
	wg := new(sync.WaitGroup)
	sig_out := make(chan bool)

	defer close(sig_out)
	defer g_wg.Done()
	if g_isOutOfOrder {
		outOfOrder(g_list)
		debug.FreeOSMemory()
	}

	start := g_list.Front()
	end := g_list.Back()
	e := g_list.Front()

	for {
		fpath, ok := e.Value.(string)
		if ok {
			// fmt.Printf("filename[%s]\n", fpath)
			var pipeline *C.GstElement // 定义组件
			var bus *C.GstBus

			v0 := GString("playbin")
			v1 := GString("play")
			pipeline = C.gst_element_factory_make(v0, v1)
			GFree(unsafe.Pointer(v0))
			GFree(unsafe.Pointer(v1))
			v2 := GString(fpath)
			C.set_path(unsafe.Pointer(pipeline), v2)
			GFree(unsafe.Pointer(v2))

			// 得到 管道的消息总线
			bus = C.pipeline_get_bus(unsafe.Pointer(pipeline))
			if bus == (*C.GstBus)(nil) {
				fmt.Println("GstBus element could not be created.Exiting.")
				return
			}
			C.bus_add_watch(unsafe.Pointer(bus), unsafe.Pointer(loop))

			C.mp3_ready(unsafe.Pointer(pipeline))
			C.mp3_play(unsafe.Pointer(pipeline))
			C.set_mute(unsafe.Pointer(pipeline))
			fmt.Printf("Playing %s\n", filepath.Base(fpath))
			wg.Add(1)
			go func(p *C.GstElement) {
				defer wg.Done()
				// 开始循环
				C.g_main_loop_run(loop)
				C.mp3_stop(unsafe.Pointer(p))
				C.object_unref(unsafe.Pointer(p))

				e = e.Next()
				if e == nil {
					sig_out <- true
				} else {
					sig_out <- false
				}

			}(pipeline)
			lb := true
			for lb {
				select {
				case op := <-cs:
					switch op {
					case 's':
						C.mp3_pause(unsafe.Pointer(pipeline))
					case 'r':
						C.mp3_play(unsafe.Pointer(pipeline))
					case 'n':
						if e != end {
							e = e.Next()
						}
						C.g_main_loop_quit(loop)
					case 'p':
						if e != start {
							e = e.Prev()
						}
						C.g_main_loop_quit(loop)
					case 'q':
						C.g_main_loop_quit(loop)
						<-sig_out
						wg.Wait()
						return
					case '+':
						g_volume_size++
						C.set_volume(unsafe.Pointer(pipeline), C.int(g_volume_size))
					case '-':
						g_volume_size--
						if g_volume_size < 0 {
							g_volume_size = 0
						}
						C.set_volume(unsafe.Pointer(pipeline), C.int(g_volume_size))
					}
				case c := <-sig_out:
					if c {
						wg.Wait()
						return
					} else {
						lb = false
					}
				}
			}
			wg.Wait()
			fmt.Printf("Finished play %s\n", filepath.Base(fpath))
		} else {
			// 路径非法
			return
		}

	}
}

func main() {
	var loop *C.GMainLoop
	mdir := ""
	mfile := ""

	flag.StringVar(&mdir, "dir", "", "mp3文件目录")
	flag.StringVar(&mfile, "file", "", "mp3文件")
	flag.BoolVar(&g_isOutOfOrder, "rand", false, "是否乱序播放")
	flag.Parse()

	if mfile != "" {
		p, err := filepath.Abs(mfile)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		C.gst_init((*C.int)(unsafe.Pointer(nil)),
			(***C.char)(unsafe.Pointer(nil)))
		loop = C.g_main_loop_new((*C.GMainContext)(unsafe.Pointer(nil)),
			C.gboolean(0)) // 创建主循环，在执行 g_main_loop_run后正式开始循环
		mfile = fmt.Sprintf("file://%s", p)

		SinglePlayProcess(mfile, loop)
		return
	}
	if mdir == "" {
		flag.PrintDefaults()
		return
	}
	g_list = list.New()
	g_wg = new(sync.WaitGroup)
	C.gst_init((*C.int)(unsafe.Pointer(nil)),
		(***C.char)(unsafe.Pointer(nil)))
	loop = C.g_main_loop_new((*C.GMainContext)(unsafe.Pointer(nil)),
		C.gboolean(0)) // 创建主循环，在执行 g_main_loop_run后正式开始循环

	if err := filepath.Walk(mdir, walkFunc); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	g_wg.Add(1)
	s := make(chan byte)
	defer close(s)
	go PlayProcess(s, loop)
	go func() {
		rd := bufio.NewReader(os.Stdin)
	LOOP0:
		for {
			s0, err := rd.ReadByte()
			if err != nil {
				fmt.Printf("%v\n", err)
			}
			//fmt.Fscanf(os.Stdin, "%c\n", &s0)
			switch s0 {
			case 's':
				s <- s0
			case 'r':
				s <- s0
			case 'n':
				s <- s0
			case 'p':
				s <- s0
			case 'q':
				s <- s0
				break LOOP0
			case 'h':
				fmt.Print("'s' -> 暂停\n" +
					"'r' -> 继续\n" +
					"'n' -> 下一首\n" +
					"'p' -> 上一首\n" +
					"'q' -> 退出\n")
			case '+':
				s <- '+'
			case '-':
				s <- '-'
			}
			s0 = 0
		}

	}()
	g_wg.Wait()
}
