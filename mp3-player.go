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
		//g_print("EOF\n");
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

static void media_ready(void *pipeline)
{
	gst_element_set_state(pipeline, GST_STATE_READY);
}

static void media_pause(void *pipeline)
{
	gst_element_set_state(pipeline, GST_STATE_PAUSED);
}

static void media_play(void *pipeline)
{
	gst_element_set_state(pipeline, GST_STATE_PLAYING);
}

static void media_stop(void *pipeline)
{
	gst_element_set_state(pipeline, GST_STATE_NULL);
}

static void set_mute(void *play)
{
	g_object_set(G_OBJECT(play), "mute", FALSE, NULL);
}

static void set_volume(void *play, int vol)
{
	int ret = vol % 101;

	g_object_set(G_OBJECT(play), "volume", ret/10.0, NULL);
}
static void media_seek(void *pipeline, gint64 pos)
{
	gint64 cpos;

	gst_element_query_position (pipeline, GST_FORMAT_TIME, &cpos);
	cpos += pos*1000*1000*1000;
	if (!gst_element_seek (pipeline, 1.0, GST_FORMAT_TIME, GST_SEEK_FLAG_FLUSH,
                         GST_SEEK_TYPE_SET, cpos,
                         GST_SEEK_TYPE_NONE, GST_CLOCK_TIME_NONE)) {
    		g_print ("Seek failed!\n");
    	}
}

*/
import "C"

import (
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

const (
        PLAY_STYLE_ORDER   = 0x100
        PLAY_STYLE_SINGLE  = 0x200
        PLAY_STYLE_SLOOP   = 0x300
        PLAY_STYLE_ALOOP   = 0x400
        PLAY_STYLE_SHUFFLE = 0x500
)

var g_list *list.List
var g_wg *sync.WaitGroup
var g_isQuit bool = false
var g_play_style int
var g_isOutOfOrder bool
var g_volume_size int = 10

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
        case ".mp3":
        case ".wav":
        case ".ogg":
        case ".wma":
        case ".rmvb":
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
        case ".mp3":
        case ".wav":
        case ".ogg":
        case ".wma":
        case ".rmvb":
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

        C.media_ready(unsafe.Pointer(pipeline))
        C.media_play(unsafe.Pointer(pipeline))

        // 开始循环
        C.g_main_loop_run(loop)
        C.media_stop(unsafe.Pointer(pipeline))
        C.object_unref(unsafe.Pointer(pipeline))
}

func PlayProcess(cs chan byte, loop *C.GMainLoop) {
        var pipeline *C.GstElement // 定义组件
        var bus *C.GstBus

        wg := new(sync.WaitGroup)
        sig_out := make(chan bool)

        g_wg.Add(1)
        defer close(sig_out)
        defer g_wg.Done()
        if g_isOutOfOrder {
                outOfOrder(g_list)
                debug.FreeOSMemory()
        }

        start := g_list.Front()
        end := g_list.Back()
        e := g_list.Front()

        v0 := GString("playbin")
        v1 := GString("play")
        pipeline = C.gst_element_factory_make(v0, v1)
        GFree(unsafe.Pointer(v0))
        GFree(unsafe.Pointer(v1))
        // 得到 管道的消息总线
        bus = C.pipeline_get_bus(unsafe.Pointer(pipeline))
        if bus == (*C.GstBus)(nil) {
                fmt.Println("GstBus element could not be created.Exiting.")
                return
        }
        C.bus_add_watch(unsafe.Pointer(bus), unsafe.Pointer(loop))
        // 开始循环

        go func(sig_quit chan bool) {
                wg.Add(1)
                i := 0
        LOOP_RUN:
                for !g_isQuit {
                        if i != 0 {
                                C.media_ready(unsafe.Pointer(pipeline))
                                C.media_play(unsafe.Pointer(pipeline))
                        }
                        C.g_main_loop_run(loop)
                        C.media_stop(unsafe.Pointer(pipeline))
                        switch g_play_style {
                        case PLAY_STYLE_SINGLE:
                                sig_quit <- true
                                break LOOP_RUN

                        case PLAY_STYLE_ORDER:
                                if e != end {
                                        e = e.Next()
                                } else {
                                        break LOOP_RUN
                                }

                        case PLAY_STYLE_SHUFFLE:
                                if e != end {
                                        e = e.Next()
                                } else {
                                        break LOOP_RUN
                                }

                        case PLAY_STYLE_SLOOP:

                        case PLAY_STYLE_ALOOP:
                                if e != end {
                                        e = e.Next()
                                } else {
                                        e = start
                                }

                        }
                        fpath, ok := e.Value.(string)
                        if ok {
                                v2 := GString(fpath)
                                C.set_path(unsafe.Pointer(pipeline), v2)
                                GFree(unsafe.Pointer(v2))

                        } else {
                                break
                        }
                        i++
                }

                C.object_unref(unsafe.Pointer(pipeline))
                wg.Done()

        }(sig_out)

        fpath, ok := e.Value.(string)
        if ok {
                // fmt.Printf("filename[%s]\n", fpath)
                v2 := GString(fpath)
                C.set_path(unsafe.Pointer(pipeline), v2)
                GFree(unsafe.Pointer(v2))

                C.media_ready(unsafe.Pointer(pipeline))
                C.media_play(unsafe.Pointer(pipeline))
                //C.set_mute(unsafe.Pointer(pipeline))

                lb := true
                for lb {
                        select {
                        case op := <-cs:
                                switch op {
                                case 's':
                                        C.media_pause(unsafe.Pointer(pipeline))
                                case 'r':
                                        C.media_play(unsafe.Pointer(pipeline))
                                case 'n':
                                        switch g_play_style {
                                        case PLAY_STYLE_SINGLE:
                                                lb = false
                                                g_isQuit = true
                                        case PLAY_STYLE_ORDER:
                                                fallthrough
                                        case PLAY_STYLE_SHUFFLE:

                                                C.media_stop(unsafe.Pointer(pipeline))
                                                if e != end {
                                                        e = e.Next()
                                                } else {
                                                        lb = false
                                                        g_isQuit = true
                                                }
                                        case PLAY_STYLE_SLOOP:
                                                C.media_stop(unsafe.Pointer(pipeline))

                                        case PLAY_STYLE_ALOOP:
                                                if e != end {
                                                        e = e.Next()
                                                } else {
                                                        e = start
                                                }

                                        }
                                        if !lb {
                                                fpath, ok := e.Value.(string)
                                                if ok {
                                                        v2 := GString(fpath)
                                                        C.set_path(unsafe.Pointer(pipeline), v2)
                                                        GFree(unsafe.Pointer(v2))
                                                        C.media_ready(unsafe.Pointer(pipeline))
                                                        C.media_play(unsafe.Pointer(pipeline))
                                                } else {
                                                        lb = false
                                                        g_isQuit = true
                                                }
                                        }
                                        //C.g_main_loop_quit(loop)
                                case 'p':
                                        switch g_play_style {
                                        case PLAY_STYLE_SINGLE:
                                                // do nothing ???
                                        case PLAY_STYLE_ORDER:
                                                fallthrough
                                        case PLAY_STYLE_SHUFFLE:

                                                C.media_stop(unsafe.Pointer(pipeline))
                                                if e != start {
                                                        e = e.Prev()
                                                        fpath, ok := e.Value.(string)
                                                        if ok {
                                                                v2 := GString(fpath)
                                                                C.set_path(unsafe.Pointer(pipeline), v2)
                                                                GFree(unsafe.Pointer(v2))
                                                                C.media_ready(unsafe.Pointer(pipeline))
                                                                C.media_play(unsafe.Pointer(pipeline))
                                                        } else {
                                                                lb = false
                                                                g_isQuit = true
                                                        }
                                                } else {
                                                        lb = false
                                                        g_isQuit = true
                                                }
                                        case PLAY_STYLE_SLOOP:
                                                C.media_stop(unsafe.Pointer(pipeline))
                                                fpath, ok := e.Value.(string)
                                                if ok {
                                                        v2 := GString(fpath)
                                                        C.set_path(unsafe.Pointer(pipeline), v2)
                                                        GFree(unsafe.Pointer(v2))
                                                        C.media_ready(unsafe.Pointer(pipeline))
                                                        C.media_play(unsafe.Pointer(pipeline))
                                                }
                                        case PLAY_STYLE_ALOOP:
                                                C.media_stop(unsafe.Pointer(pipeline))
                                                if e != start {
                                                        e = e.Prev()
                                                } else {
                                                        e = end
                                                }
                                                fpath, ok := e.Value.(string)
                                                if ok {
                                                        v2 := GString(fpath)
                                                        C.set_path(unsafe.Pointer(pipeline), v2)
                                                        GFree(unsafe.Pointer(v2))
                                                        C.media_ready(unsafe.Pointer(pipeline))
                                                        C.media_play(unsafe.Pointer(pipeline))
                                                }
                                        }

                                case 'q':
                                        lb = false
                                        g_isQuit = true
                                case '+':
                                        g_volume_size++
                                        C.set_volume(unsafe.Pointer(pipeline), C.int(g_volume_size))
                                case '-':
                                        g_volume_size--
                                        if g_volume_size < 0 {
                                                g_volume_size = 0
                                        }
                                        C.set_volume(unsafe.Pointer(pipeline), C.int(g_volume_size))
                                case 't':
                                        C.media_seek(unsafe.Pointer(pipeline), C.gint64(5))

                                }
                        case vv0 := <-sig_out:
                                if vv0 {
                                        C.g_main_loop_quit(loop)
                                        wg.Wait()
                                        g_wg.Done()
                                        g_wg.Wait()
                                        close(sig_out)
                                        os.Exit(0)
                                }
                        }
                }

        } else {
                // 路径非法
                return
        }

        C.g_main_loop_quit(loop)
        wg.Wait()

}

func main() {
        var loop *C.GMainLoop
        var s0 byte
        mdir := ""
        mfile := ""
        style := ""

        flag.StringVar(&mdir, "dir", "", "mp3文件目录")
        flag.StringVar(&mfile, "file", "", "mp3文件")
        flag.StringVar(&style, "style", "order", "播放方式[顺序:order|乱序:shuffle|单曲:single|单曲循环:sloop|全部循环:aloop]")
        flag.Parse()

        switch style {
        case "shuffle":
                g_isOutOfOrder = true
                g_play_style = PLAY_STYLE_SHUFFLE

        case "order":
                g_play_style = PLAY_STYLE_ORDER
        case "single":
                g_play_style = PLAY_STYLE_SINGLE
        case "sloop":
                g_play_style = PLAY_STYLE_SLOOP
        case "aloop":
                g_play_style = PLAY_STYLE_ALOOP
        default:
                flag.PrintDefaults()
                return
        }
        g_list = list.New()
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
                g_list.PushBack(mfile)
                g_play_style = PLAY_STYLE_SINGLE
        } else {
                if mdir == "" {
                        flag.PrintDefaults()
                        return
                }
                if err := filepath.Walk(mdir, walkFunc); err != nil {
                        fmt.Printf("Error: %v\n", err)
                        return
                }
        }

        g_wg = new(sync.WaitGroup)
        C.gst_init((*C.int)(unsafe.Pointer(nil)),
                (***C.char)(unsafe.Pointer(nil)))
        loop = C.g_main_loop_new((*C.GMainContext)(unsafe.Pointer(nil)),
                C.gboolean(0)) // 创建主循环，在执行 g_main_loop_run后正式开始循环

        s := make(chan byte)
        defer close(s)
        go PlayProcess(s, loop)

        isQuit := false
        for !isQuit {
                fmt.Fscanf(os.Stdin, "%c\n", &s0)
                switch s0 {
                case 's':
                        fallthrough
                case 'r':
                        fallthrough
                case 'n':
                        fallthrough
                case 'p':
                        fallthrough
                case 't':
                        fallthrough
                case '+':
                        fallthrough
                case '-':
                        s <- s0
                case 'q':
                        s <- s0
                        isQuit = true
                case 'h':
                        fmt.Print("'s' -> 暂停\n" +
                                "'r' -> 继续\n" +
                                "'n' -> 下一首\n" +
                                "'p' -> 上一首\n" +
                                "'q' -> 退出\n")
                }
                s0 = 0
        }

        g_wg.Wait()

}
