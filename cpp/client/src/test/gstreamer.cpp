#include "gst/gst.h"
#include "gst/app/gstappsrc.h"

#include "boost/format.hpp"

#include "cpptrace/cpptrace.hpp"

#include "Poco/URI.h"
#include "Poco/Net/HTTPClientSession.h"
#include "Poco/Net/HTTPRequest.h"
#include "Poco/Net/HTTPResponse.h"

#include "spdlog/spdlog.h"

#include "cfgo/client.hpp"
#include "cfgo/track.hpp"
#include "cfgo/subscribation.hpp"
#include "cfgo/async.hpp"

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
    guint watchid;

    GTimer *timer;
    cfgo::Track::Ptr m_track;
    bool m_valid;

    App(const cfgo::Track::Ptr & track, GMainLoop *loop): m_track(track), m_loop(loop) {
        timer = g_timer_new();
        sourceid = 0;
        watchid = 0;

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
        watchid = gst_bus_add_watch(bus, (GstBusFunc) on_bus_message, this);
        g_signal_connect(appsrc, "need-data", G_CALLBACK(start_feed), this);
        g_signal_connect(appsrc, "enough-data", G_CALLBACK(stop_feed), this);
    }

    void run() {
        gst_element_set_state (pipeline, GST_STATE_PLAYING);
    }

    ~App() {
        if (m_valid)
        {
            gst_element_set_state( pipeline, GST_STATE_NULL);
            gst_object_unref(pipeline);
            g_source_remove(sourceid);
        }
    } 
};

static gboolean read_data(App *app)
{
    auto track = app->m_track->track();
    if (track->isOpen())
    {
        if (auto msg = track->receive(); msg.has_value())
        {
            if (std::holds_alternative<rtc::binary>(*msg))
            {
                GstBuffer *buffer;

                auto && data = std::get<rtc::binary>(*msg);

                buffer = gst_buffer_new_and_alloc(data.size());
                GstMapInfo info = GST_MAP_INFO_INIT;
                if (!gst_buffer_map(buffer, &info, GST_MAP_READWRITE))
                {
                    return FALSE;
                }
                memcpy(info.data, data.data(), data.size());
                gst_buffer_unmap(buffer, &info);
                GstFlowReturn ret;
                g_signal_emit_by_name(app->appsrc, "push-buffer", buffer, &ret);                
                gst_buffer_unref(buffer);
                return ret== GST_FLOW_OK;
            }
            
        }
        
    }
    
    return FALSE;
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

auto get_token() -> std::string {
    using namespace Poco::Net;
    Poco::URI uri("http://localhost:3100/token?key=10000&uid=10000&uname=user10000&role=parent&room=room0&nonce=12345&autojoin=true");
    HTTPClientSession session(uri.getHost(), uri.getPort());
    HTTPRequest request(HTTPRequest::HTTP_GET, uri.getPathAndQuery(), HTTPMessage::HTTP_1_1);
    HTTPResponse response;
    session.sendRequest(request);
    auto&& rs = session.receiveResponse(response);
    if (response.getStatus() != HTTPResponse::HTTP_OK)
    {
        throw cpptrace::runtime_error((boost::format("unable to get the token. status code: %1%.") % response.getStatus()).str());
    }
    return std::string{ std::istreambuf_iterator<char>(rs), std::istreambuf_iterator<char>() };
}

auto main_task(const std::string & token, const cfgo::Client::CtxPtr & io_ctx, GMainLoop *loop) -> asio::awaitable<void> {
    std::cout << "start main task." << std::endl;
    cfgo::Configuration conf { "http://localhost:8080", token };
    cfgo::Client client(conf, io_ctx);
    // client.set_sio_logs_verbose();
    std::cout << "client created." << std::endl;
    cfgo::Pattern pattern {
        cfgo::Pattern::Op::ALL,
        {},
        {
            cfgo::Pattern {
                cfgo::Pattern::Op::TRACK_TYPE,
                { "video" },
                {}
            },
            cfgo::Pattern {
                cfgo::Pattern::Op::TRACK_LABEL_ALL_MATCH,
                { "uid", "0" },
                {}
            }
        }
    };
    auto timeout = co_await cfgo::make_timeout(std::chrono::seconds{30});
    std::cout << "subscribing..." << std::endl;
    auto sub = co_await client.subscribe(pattern, {}, timeout);
    if (!sub)
    {
        throw cpptrace::runtime_error("sub timeout");
    }
    std::cout << "subscribed successfully with sub id: " << sub->sub_id() << ", pub id: " << sub->pub_id() << std::endl;
    App app {sub->tracks()[0], loop};
    app.run();
    co_await cfgo::wait_timeout(std::chrono::seconds{120});
}

int main(int argc, char **argv) {

    gst_init(&argc, &argv);
    GST_DEBUG_CATEGORY_INIT (appsrc_pipeline_debug, "appsrc-pipeline", 1, "appsrc pipeline example");
    spdlog::set_level(spdlog::level::debug);
    auto token = get_token();
    std::cout << "got token: " << token << std::endl;
    auto pool = std::make_shared<asio::thread_pool>();
    GMainLoop *loop = g_main_loop_new(NULL, TRUE);
    asio::co_spawn(*pool, main_task(token, pool, loop), asio::detached);
    g_main_loop_run(loop);
    GST_DEBUG("stopping");
    g_main_loop_unref(loop);
}