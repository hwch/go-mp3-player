package main

/*
#cgo pkg-config: gstreamer-1.0

#include <linux/fb.h>
#include <sys/mman.h>
#include <unistd.h>
#include <time.h>
#include <fcntl.h>
#include <sys/ioctl.h>
#include <string.h>
#include <sys/wait.h>
#include <signal.h>
#include <math.h>
#include <stdio.h>
#include <sys/types.h>
#include <dirent.h>
#include <stdlib.h>
#include <errno.h>
#include <glib.h>
#include <gst/gst.h>


// ******************** 定义消息处理函数 ********************
gboolean bus_call(GstBus *bus, GstMessage *msg, gpointer data)
{
        GMainLoop *loop = (GMainLoop *)data;//这个是主循环的指针，在接受EOS消息时退出循环
        gchar *debug;
        GError *error;

        switch (GST_MESSAGE_TYPE(msg)) {
        case GST_MESSAGE_EOS:
                // g_print("End of stream\n");
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

static void obj_set(void *d, void *s0, void *s1)
{
        g_object_set(G_OBJECT(d), s0, s1, NULL);
}

static void bin_add_many(void *bin, void *v0, void *v1, void *v2)
{
        gst_bin_add_many(GST_BIN(bin), v0, v1, v2, NULL);
}
static GstBus *pipeline_get_bus(void *pipeline)
{
        return gst_pipeline_get_bus(GST_PIPELINE(pipeline));
}

static void element_link_many(void *source, void *decoder, void *sink)
{
        gst_element_link_many(source, decoder, sink, NULL);
}

static void bus_add_watch(void *bus, void *loop)
{
        gst_bus_add_watch(bus, bus_call, loop);
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

*/
import "C"

import (
        "container/list"
        "flag"
        "fmt"
        "os"
        "path/filepath"
        "sync"
        "unsafe"
)

const MP3_FILE_MAX = 10

var g_list *list.List
var g_wg *sync.WaitGroup

func GString(s string) *C.gchar {
        return (*C.gchar)(C.CString(s))
}

func GFree(s C.gpointer) {
        C.g_free(s)
}

func walkFunc(fpath string, info os.FileInfo, err error) error {
        if info.IsDir() {
                return nil
        }
        switch filepath.Ext(fpath) {
        case ".mp3":
        case ".wav":
        case ".ogg":
                /* v3 := GString("oggparse")
                   defer GFree(C.gpointer(v3))
                   v4 := GString("ogg-parser")
                   defer GFree(C.gpointer(v4))
                   decoder = C.gst_element_factory_make(v3, v4) */
                fallthrough
        default:
                return nil
        }
        g_list.PushBack(fpath)
        return err
}

func mp3_play_process(cs chan byte, loop *C.GMainLoop) {
        wg := new(sync.WaitGroup)
        sig_in := make(chan bool)
        sig_out := make(chan bool)

        defer close(sig_in)
        defer close(sig_out)
        defer g_wg.Done()
        start := g_list.Front()
        end := g_list.Back()
        e := g_list.Front()

        for {
                fpath, ok := e.Value.(string)
                if ok {
                        // fmt.Printf("filename[%s]\n", fpath)
                        var pipeline, source, decoder, sink *C.GstElement // 定义组件
                        var bus *C.GstBus

                        switch filepath.Ext(fpath) {
                        case ".mp3":
                                v3 := GString("mad")
                                v4 := GString("mad-decoder")
                                decoder = C.gst_element_factory_make(v3, v4)
                                GFree(C.gpointer(v4))
                                GFree(C.gpointer(v3))
                        case ".wav":
                                v3 := GString("wavparse")
                                v4 := GString("parser")
                                decoder = C.gst_element_factory_make(v3, v4)
                                GFree(C.gpointer(v3))
                                GFree(C.gpointer(v4))
                        case ".ogg":
                                // v3 := GString("oggparse")
                                // defer GFree(C.gpointer(v3))
                                // v4 := GString("ogg-parser")
                                // defer GFree(C.gpointer(v4))
                                // decoder = C.gst_element_factory_make(v3, v4)
                                fallthrough
                        default:
                                return
                        }

                        // fmt.Printf("FileName[%s]FileSize[%d]Dir[%v]\n", path, info.Size(), info.IsDir())

                        // 创建管道和组件
                        v0 := GString("audio-player")
                        pipeline = C.gst_pipeline_new(v0)
                        GFree(C.gpointer(v0))
                        v1 := GString("filesrc")
                        v2 := GString("file-source")
                        source = C.gst_element_factory_make(v1, v2)
                        GFree(C.gpointer(v1))
                        GFree(C.gpointer(v2))

                        // sink = gst_element_factory_make("autoaudiosink","audio-output");
                        v5 := GString("alsasink")
                        v6 := GString("alsa-output")
                        sink = C.gst_element_factory_make(v5, v6)
                        GFree(C.gpointer(v6))
                        GFree(C.gpointer(v5))
                        if pipeline == (*C.GstElement)(nil) ||
                                source == (*C.GstElement)(nil) ||
                                decoder == (*C.GstElement)(nil) ||
                                sink == (*C.GstElement)(nil) {
                                fmt.Println("One element could not be created.Exiting.")
                        }

                        // 设置 source的location 参数。即 文件地址.
                        v7 := GString("location")
                        cpath := GString(fpath)
                        C.obj_set(unsafe.Pointer(source),
                                unsafe.Pointer(v7), unsafe.Pointer(cpath))
                        GFree(C.gpointer(v7))
                        GFree(C.gpointer(cpath))

                        // 得到 管道的消息总线
                        bus = C.pipeline_get_bus(unsafe.Pointer(pipeline))
                        if bus == (*C.GstBus)(nil) {
                                fmt.Println("GstBus element could not be created.Exiting.")
                                return
                        }

                        // 添加消息监视器
                        C.bus_add_watch(unsafe.Pointer(bus), unsafe.Pointer(loop))
                        C.gst_object_unref(C.gpointer(bus))

                        // 把组件添加到管道中.管道是一个特殊的组件，可以更好的让数据流动
                        C.bin_add_many(unsafe.Pointer(pipeline),
                                unsafe.Pointer(source), unsafe.Pointer(decoder),
                                unsafe.Pointer(sink))

                        // 依次连接组件
                        C.element_link_many(unsafe.Pointer(source),
                                unsafe.Pointer(decoder), unsafe.Pointer(sink))

                        // 开始播放
                        C.mp3_ready(unsafe.Pointer(pipeline))
                        C.mp3_play(unsafe.Pointer(pipeline))

                        wg.Add(1)
                        go func() {
                                defer wg.Done()
                                // 开始循环
                                C.g_main_loop_run(loop)
                                C.mp3_stop(unsafe.Pointer(pipeline))
                                C.object_unref(unsafe.Pointer(pipeline))

                                select {
                                case c := <-sig_in:
                                        if c {
                                                return
                                        }
                                default:
                                        e = e.Next()
                                        if e == nil {
                                                sig_out <- true
                                                return
                                        }
                                }
                                sig_out <- false
                        }()
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
                                                sig_in <- false
                                        case 'p':
                                                if e != start {
                                                        e = e.Prev()
                                                }
                                                C.g_main_loop_quit(loop)
                                                sig_in <- false
                                        case 'q':
                                                C.g_main_loop_quit(loop)
                                                sig_in <- true
                                                wg.Wait()
                                                return
                                        }
                                case c := <-sig_out:
                                        if c {
                                                return
                                        } else {
                                                lb = false
                                        }
                                }
                        }
                        wg.Wait()
                }

        }
}

func main() {
        var loop *C.GMainLoop
        var s0 byte
        mdir := ""

        flag.StringVar(&mdir, "mdir", "", "mp3文件目录")
        flag.Parse()

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
        go mp3_play_process(s, loop)
LOOP0:
        for {
                fmt.Fscanf(os.Stdin, "%c\n", &s0)
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
                }
                s0 = 0
        }
        g_wg.Wait()
}
