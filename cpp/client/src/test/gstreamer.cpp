#include "gst/gst.h"
#include "gst/app/gstappsrc.h"


#include "cfgo/client.hpp"
#include "cfgo/track.hpp"

GST_DEBUG_CATEGORY_STATIC (appsrc_pipeline_debug);
#define GST_CAT_DEFAULT appsrc_pipeline_debug

#define CHECK_ELEMENT_CREATE(var_name) \
if(!var_name) { \
    g_printerr("The element %s can not be created.\n", #var_name); \
    m_valid = false; \
    return; \
}

struct App;

static void start_feed(GstElement *pipeline, guint size, App *app);
static void stop_feed(GstElement *pipeline, App *app);
static gboolean on_bus_message(GstBus *bus, GstMessage *message, App *app);

struct App
{
    GstElement *pipeline;
    GstElement *appsrc;
    GstElement *rtpbin;
    GstElement *decodebin;
    GstElement *videoconvert;
    GstElement *filesink;

    GstBus *bus;

    GMainLoop *m_loop;
    guint sourceid;

    GTimer *timer;
    cfgo::Track::Ptr m_track;
    bool m_valid;

    App(const cfgo::Track::Ptr & track, GMainLoop *loop): m_track(track), m_loop(loop) {
        timer = g_timer_new();

        pipeline = gst_pipeline_new("test-pipeline");
        CHECK_ELEMENT_CREATE(pipeline)
        appsrc = gst_element_factory_make("appsrc", "appsrc");
        CHECK_ELEMENT_CREATE(appsrc)
        rtpbin = gst_element_factory_make("rtpbin", "rtpbin");
        CHECK_ELEMENT_CREATE(rtpbin)
        decodebin = gst_element_factory_make("decodebin3", "decodebin");
        CHECK_ELEMENT_CREATE(decodebin)
        videoconvert = gst_element_factory_make("videoconvert", "videoconvert");
        CHECK_ELEMENT_CREATE(videoconvert)
        filesink = gst_element_factory_make("filesink", "filesink");
        CHECK_ELEMENT_CREATE(filesink)
        
        gst_bin_add_many(GST_BIN(pipeline), appsrc, rtpbin, decodebin, videoconvert, filesink, NULL);
        if (!gst_element_link_many(appsrc, rtpbin, decodebin, videoconvert, filesink, NULL))
        {
            g_printerr("Elements could not be linked.\n");
            m_valid = false;
            return;
        }

        bus = gst_pipeline_get_bus(GST_PIPELINE(pipeline));
        if (!bus)
        {
            g_printerr("Unable to achieve the bus.");
            m_valid = false;
            return;
        }
        gst_bus_add_watch(bus, (GstBusFunc) on_bus_message, this);
        g_signal_connect(appsrc, "need-data", G_CALLBACK(start_feed), this);
        g_signal_connect(appsrc, "enough-data", G_CALLBACK(stop_feed), this);
    }

    void run() {
        gst_element_set_state (pipeline, GST_STATE_PLAYING);
    }

    ~App() {
        gst_object_unref(pipeline);
    } 
};

static gboolean read_data(App *app)
{
    auto track = app->m_track->track();
    if (track->isOpen())
    {
    }
    return TRUE;
}

static void start_feed(GstElement *pipeline, guint size, App *app) 
{
    if (app->sourceid == 0)
    {
        GST_DEBUG("start feeding");
        app->sourceid = g_idle_add((GSourceFunc) read_data, app);
    }
    
}

static void stop_feed(GstElement *pipeline, App *app) 
{
    if (app->sourceid != 0)
    {
        GST_DEBUG("stop feeding");
        g_source_remove(app->sourceid);
        app->sourceid = 0;
    }
    
}

static gboolean on_bus_message(GstBus *bus, GstMessage *message, App *app) 
{
    return TRUE;
}

int main(int argc, char **argv) {

    gst_init(&argc, &argv);

    GST_DEBUG_CATEGORY_INIT (appsrc_pipeline_debug, "appsrc-pipeline", 1, "appsrc pipeline example");

}