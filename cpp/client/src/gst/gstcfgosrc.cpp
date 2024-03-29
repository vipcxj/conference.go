/**
 * SECTION:element-gstcfgosrc
 *
 * The cfgosrc element does FIXME stuff.
 *
 * <refsect2>
 * <title>Example launch line</title>
 * |[
 * gst-launch-1.0 -v fakesrc ! cfgosrc ! FIXME ! fakesink
 * ]|
 * FIXME Describe what the pipeline does.
 * </refsect2>
 */

#ifdef HAVE_CONFIG_H
#include "config.h"
#endif

#include <gst/gst.h>
#include "cfgo/gst/gstcfgosrc.h"
#include "cfgo/gst/cfgosrc.hpp"
#include "cfgo/gst/error.hpp"
#include "cfgo/gst/utils.hpp"
#include "cfgo/async.hpp"
#include "cfgo/capi.h"
#include "cfgo/cbridge.hpp"
#include "cfgo/client.hpp"
#include "cfgo/defer.hpp"
#include "asio.hpp"

GST_DEBUG_CATEGORY_STATIC(gst_cfgosrc_debug_category);
#define GST_CAT_DEFAULT gst_cfgosrc_debug_category

/* prototypes */

namespace cfgo
{
    namespace gst
    {
        struct GstCfgoSrcPrivateState
        {
            std::mutex mutex;
            bool running;
            cfgo::gst::CfgoSrc::Ptr task;
        };
    } // namespace gst
    
} // namespace cfgo


struct _GstCfgoSrcPrivate
{
    cfgo::gst::GstCfgoSrcPrivateState * state;
};

#define GST_CFGOSRC_PVS(cfgosrc) cfgosrc->priv->state

#define GST_CFGOSRC_ACTUALLY_JOIN(x, y) x##y
#define GST_CFGOSRC_JOIN(x, y) GST_CFGOSRC_ACTUALLY_JOIN(x, y)
#ifdef __COUNTER__
#define GST_CFGOSRC_LOCK_GUARD_VARNAME(x) GST_CFGOSRC_JOIN(x, __COUNTER__)
#else
#define GST_CFGOSRC_LOCK_GUARD_VARNAME(x) GST_CFGOSRC_JOIN(x, __LINE__)
#endif
#define GST_CFGOSRC_LOCK_GUARD(cfgosrc) std::lock_guard GST_CFGOSRC_LOCK_GUARD_VARNAME(guard)(GST_CFGOSRC_PVS(cfgosrc)->mutex)

static void gst_cfgosrc_set_property(GObject *object,
                                     guint property_id, const GValue *value, GParamSpec *pspec);
static void gst_cfgosrc_get_property(GObject *object,
                                     guint property_id, GValue *value, GParamSpec *pspec);
static void gst_cfgosrc_dispose(GObject *object);
static void gst_cfgosrc_finalize(GObject *object);

static GstPad *gst_cfgosrc_request_new_pad(GstElement *element,
                                           GstPadTemplate *templ, const gchar *name);
static void gst_cfgosrc_release_pad(GstElement *element, GstPad *pad);
static GstStateChangeReturn
gst_cfgosrc_change_state(GstElement *element, GstStateChange transition);

enum
{
    PROP_0,
    PROP_CLIENT,
    PROP_PATTERN,
    PROP_REQ_TYPES,
    PROP_SUB_TIMEOUT,
    PROP_SUB_TRIES,
    PROP_SUB_TRY_DELAY_INIT,
    PROP_SUB_TRY_DELAY_STEP,
    PROP_SUB_TRY_DELAY_LEVEL,
    PROP_READ_TIMEOUT,
    PROP_READ_TRIES,
    PROP_READ_TRY_DELAY_INIT,
    PROP_READ_TRY_DELAY_STEP,
    PROP_READ_TRY_DELAY_LEVEL
};

/* pad templates */

static GstStaticPadTemplate gst_cfgosrc_rtp_src_template =
    GST_STATIC_PAD_TEMPLATE("rtp_src_%u_%u",
                            GST_PAD_SRC,
                            GST_PAD_SOMETIMES,
                            GST_STATIC_CAPS("application/x-rtp"));

/* class initialization */

G_DEFINE_TYPE_WITH_CODE(GstCfgoSrc, gst_cfgosrc, GST_TYPE_BIN,
    GST_DEBUG_CATEGORY_INIT(
        gst_cfgosrc_debug_category, "cfgosrc", 0,
        "debug category for cfgosrc element"
    );
    G_ADD_PRIVATE(GstCfgoSrc)
);

static void
gst_cfgosrc_class_init(GstCfgoSrcClass *klass)
{
    GObjectClass *gobject_class = G_OBJECT_CLASS(klass);
    GstElementClass *element_class = GST_ELEMENT_CLASS(klass);

    /* Setting up pads and setting metadata should be moved to
       base_class_init if you intend to subclass this class. */
    gst_element_class_add_static_pad_template(element_class,
                                              &gst_cfgosrc_rtp_src_template);

    gst_element_class_set_static_metadata(GST_ELEMENT_CLASS(klass),
                                          "FIXME Long name", "Generic", "FIXME Description",
                                          "FIXME <fixme@example.com>");

    gobject_class->set_property = gst_cfgosrc_set_property;
    gobject_class->get_property = gst_cfgosrc_get_property;
    gobject_class->dispose = gst_cfgosrc_dispose;
    gobject_class->finalize = gst_cfgosrc_finalize;
    element_class->change_state = GST_DEBUG_FUNCPTR(gst_cfgosrc_change_state);

    g_object_class_install_property(
        gobject_class, PROP_CLIENT,
        g_param_spec_int(
            "client", "client", "The cfgo client",
            0, G_MAXINT, 0,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_PATTERN,
        g_param_spec_string(
            "pattern", "pattern", "The pattern for subscribing",
            NULL,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_REQ_TYPES,
        g_param_spec_string(
            "req-types", "req-types", "The reqire types for subscribing, \"video\", \"audio\" or \"video,audio\"",
            NULL,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_SUB_TIMEOUT,
        g_param_spec_uint64(
            "sub-timeout", "sub-timeout", "timeout miliseconds of subscribing",
            0, G_MAXUINT64, 0,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_SUB_TRIES,
        g_param_spec_int(
            "sub-tries", "sub-tries", "max tries of subscribing, -1 means forever",
            -1, G_MAXINT32, -1,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_SUB_TRY_DELAY_INIT,
        g_param_spec_uint64(
            "sub-try-delay-init", "sub-try-delay-init", "the init delay time in milisecond before next subscribing try",
            0, G_MAXUINT64, 0,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_SUB_TRY_DELAY_STEP,
        g_param_spec_uint(
            "sub-try-delay-step", "sub-try-delay-step", "the increase step of delay time in milisecond before next subscribing try.",
            0, G_MAXINT32, 0,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_SUB_TRY_DELAY_LEVEL,
        g_param_spec_uint(
            "sub-try-delay-level", "sub-try-delay-level", "the max increase level of delay time before next subscribing try.",
            0, 16, 0,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_READ_TIMEOUT,
        g_param_spec_uint64(
            "read-timeout", "read-timeout", "timeout miliseconds of reading data",
            0, G_MAXUINT64, 0,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_READ_TRIES,
        g_param_spec_int(
            "read-tries", "read-tries", "max tries of reading data, -1 means forever",
            -1, G_MAXINT32, -1,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_READ_TRY_DELAY_INIT,
        g_param_spec_uint64(
            "read-try-delay-init", "read-try-delay-init", "the init delay time in milisecond before next reading data try",
            0, G_MAXUINT64, 0,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_READ_TRY_DELAY_STEP,
        g_param_spec_uint(
            "read-try-delay-step", "read-try-delay-step", "the increase step of delay time in milisecond before next reading data try.",
            0, G_MAXINT32, 0,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_READ_TRY_DELAY_LEVEL,
        g_param_spec_uint(
            "read-try-delay-level", "read-try-delay-level", "the max increase level of delay time before next reading data try.",
            0, 16, 0,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
}

static void
gst_cfgosrc_init(GstCfgoSrc *cfgosrc)
{
    cfgosrc->priv = (GstCfgoSrcPrivate *)gst_cfgosrc_get_instance_private(cfgosrc);
    cfgosrc->priv->state = new cfgo::gst::GstCfgoSrcPrivateState();
}

void _gst_cfgosrc_prepare(GstCfgoSrc *cfgosrc, bool reset_task)
{
    if (GST_CFGOSRC_PVS(cfgosrc)->running)
    {
        if (cfgosrc->client_handle > 0 
            && cfgosrc->pattern && strlen(cfgosrc->pattern) > 0)
        {
            if (!GST_CFGOSRC_PVS(cfgosrc)->task || reset_task)
            {
                GST_DEBUG_OBJECT(cfgosrc, "%s", "Creating the task.");
                GST_CFGOSRC_PVS(cfgosrc)->task = cfgo::gst::CfgoSrc::create(
                    cfgosrc->client_handle, 
                    cfgosrc->pattern, cfgosrc->req_types, 
                    cfgosrc->sub_timeout, cfgosrc->read_timeout
                );
                GST_DEBUG_OBJECT(cfgosrc, "%s", "Attaching the task.");
                GST_CFGOSRC_PVS(cfgosrc)->task->attach(cfgosrc);
                GST_DEBUG_OBJECT(cfgosrc, "%s", "Starting the task.");
                GST_CFGOSRC_PVS(cfgosrc)->task->start();
            }
            else
            {
                GST_DEBUG_OBJECT(cfgosrc, "%s", "The task has created and started, updating timeout properties.");
                GST_CFGOSRC_PVS(cfgosrc)->task->set_sub_timeout(cfgosrc->sub_timeout);
                GST_CFGOSRC_PVS(cfgosrc)->task->set_sub_timeout(cfgosrc->read_timeout);
            }
            GST_DEBUG_OBJECT(cfgosrc, "%s", "Updating the try options of the task.");
            GST_CFGOSRC_PVS(cfgosrc)->task->set_sub_try(
                cfgosrc->sub_tries,
                cfgosrc->sub_try_delay_init,
                cfgosrc->sub_try_delay_step,
                cfgosrc->sub_try_delay_level
            );
            GST_CFGOSRC_PVS(cfgosrc)->task->set_read_try(
                cfgosrc->read_tries,
                cfgosrc->read_try_delay_init,
                cfgosrc->read_try_delay_step,
                cfgosrc->read_try_delay_level
            );
        }
    }
}

void gst_cfgosrc_start(GstCfgoSrc *cfgosrc)
{
    GST_DEBUG_OBJECT(cfgosrc, "%s", "gst_cfgosrc_start");
    GST_CFGOSRC_LOCK_GUARD(cfgosrc);
    GST_CFGOSRC_PVS(cfgosrc)->running = true;
    _gst_cfgosrc_prepare(cfgosrc, false);
    if (GST_CFGOSRC_PVS(cfgosrc)->task)
    {
        GST_DEBUG_OBJECT(cfgosrc, "%s", "resume task");
        GST_CFGOSRC_PVS(cfgosrc)->task->resume();
    }
}

void gst_cfgosrc_stop(GstCfgoSrc *cfgosrc)
{
    GST_CFGOSRC_LOCK_GUARD(cfgosrc);
    GST_CFGOSRC_PVS(cfgosrc)->running = false;
    if (GST_CFGOSRC_PVS(cfgosrc)->task)
    {
        GST_CFGOSRC_PVS(cfgosrc)->task->pause();
    }
}

bool gst_cfgosrc_set_string_property(const GValue *value, const gchar ** target)
{
    const gchar * src = g_value_get_string(value);
    if (src == *target)
    {
        return false;
    }
    else if (src == nullptr)
    {
        g_free(target);
        *target = nullptr;
        return true;
    }
    else if (*target == nullptr)
    {
        *target = g_strdup(src);
        return true;
    }
    else if (g_str_equal(src, *target))
    {
        return false;
    }
    else
    {
        g_free(target);
        *target = g_strdup(src);
        return true;
    }
}

bool gst_cfgosrc_set_int32_property(const GValue * value, gint32 * target)
{
    auto src = g_value_get_int(value);
    if (src == *target)
    {
        return false;
    }
    else
    {
        *target = src;
        return true;
    }
}

bool gst_cfgosrc_set_uint32_property(const GValue * value, guint32 * target)
{
    auto src = g_value_get_uint(value);
    if (src == *target)
    {
        return false;
    }
    else
    {
        *target = src;
        return true;
    }
}

bool gst_cfgosrc_set_uint64_property(const GValue * value, guint64 * target)
{
    auto src = g_value_get_uint64(value);
    if (src == *target)
    {
        return false;
    }
    else
    {
        *target = src;
        return true;
    }
}

void gst_cfgosrc_set_property(GObject *object, guint property_id,
                              const GValue *value, GParamSpec *pspec)
{
    GstCfgoSrc *cfgosrc = GST_CFGOSRC(object);

    GST_DEBUG_OBJECT(cfgosrc, "set_property");

    switch (property_id)
    {
    case PROP_CLIENT:
    {
        int handle = g_value_get_int(value);
        if (handle > 0)
        {
            GST_CFGOSRC_LOCK_GUARD(cfgosrc);
            if (handle == cfgosrc->client_handle)
            {
                break;
            }
            if (cfgosrc->client_handle > 0)
            {
                cfgo_client_unref(cfgosrc->client_handle);
            }
            cfgo_client_ref(handle);
            cfgosrc->client_handle = handle;
            GST_DEBUG_OBJECT(cfgosrc, "Client argument was changed to %d\n", handle);
            _gst_cfgosrc_prepare(cfgosrc, true);
        }
        else
        {
            GST_WARNING_OBJECT(cfgosrc, "Accept an invalid client handle: %d.\n", handle);
        }
        break;
    }
    case PROP_PATTERN:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_string_property(value, &cfgosrc->pattern))
        {
             _gst_cfgosrc_prepare(cfgosrc, true);
            GST_DEBUG_OBJECT(cfgosrc, "The pattern argument was changed to %s\n", cfgosrc->pattern);
        }
        break;
    }
    case PROP_REQ_TYPES:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_string_property(value, &cfgosrc->req_types))
        {
             _gst_cfgosrc_prepare(cfgosrc, true);
            GST_DEBUG_OBJECT(cfgosrc, "The req-types argument was changed to %s\n", cfgosrc->req_types);
        }
        break;
    }
    case PROP_SUB_TIMEOUT:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_uint64_property(value, &cfgosrc->sub_timeout))
        {
            GST_DEBUG_OBJECT(cfgosrc, "The sub-timeout argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->sub_timeout));
        }
        break;
    }
    case PROP_SUB_TRIES:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_int32_property(value, &cfgosrc->sub_tries))
        {
            GST_DEBUG_OBJECT(cfgosrc, "The sub-tries argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->sub_tries));
        }
        break;
    }
    case PROP_SUB_TRY_DELAY_INIT:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_uint64_property(value, &cfgosrc->sub_try_delay_init))
        {
            GST_DEBUG_OBJECT(cfgosrc, "The sub-try-delay-init argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->sub_try_delay_init));
        }
        break;
    }
    case PROP_SUB_TRY_DELAY_STEP:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_uint32_property(value, &cfgosrc->sub_try_delay_step))
        {
            GST_DEBUG_OBJECT(cfgosrc, "The sub-try-delay-step argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->sub_try_delay_step));
        }
        break;
    }
    case PROP_SUB_TRY_DELAY_LEVEL:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_uint32_property(value, &cfgosrc->sub_try_delay_level))
        {
            GST_DEBUG_OBJECT(cfgosrc, "The sub-try-delay-level argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->sub_try_delay_level));
        }
        break;
    }
    case PROP_READ_TIMEOUT:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_uint64_property(value, &cfgosrc->read_timeout))
        {
            GST_DEBUG_OBJECT(cfgosrc, "The read-timeout argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->read_timeout));
        }
        break;
    }
    case PROP_READ_TRIES:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_int32_property(value, &cfgosrc->read_tries))
        {
            GST_DEBUG_OBJECT(cfgosrc, "The read-tries argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->read_tries));
        }
        break;
    }
    case PROP_READ_TRY_DELAY_INIT:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_uint64_property(value, &cfgosrc->read_try_delay_init))
        {
            GST_DEBUG_OBJECT(cfgosrc, "The read-try-delay-init argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->read_try_delay_init));
        }
        break;
    }
    case PROP_READ_TRY_DELAY_STEP:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_uint32_property(value, &cfgosrc->read_try_delay_step))
        {
            GST_DEBUG_OBJECT(cfgosrc, "The read-try-delay-step argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->read_try_delay_step));
        }
        break;
    }
    case PROP_READ_TRY_DELAY_LEVEL:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (gst_cfgosrc_set_uint32_property(value, &cfgosrc->read_try_delay_level))
        {
            GST_DEBUG_OBJECT(cfgosrc, "The read-try-delay-level argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->read_try_delay_level));
        }
        break;
    }
    default:
        G_OBJECT_WARN_INVALID_PROPERTY_ID(object, property_id, pspec);
        break;
    }
}

void gst_cfgosrc_get_property(GObject *object, guint property_id,
                              GValue *value, GParamSpec *pspec)
{
    GstCfgoSrc *cfgosrc = GST_CFGOSRC(object);

    GST_DEBUG_OBJECT(cfgosrc, "get_property");

    switch (property_id)
    {
    case PROP_CLIENT:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_int(value, cfgosrc->client_handle);
        break;
    }
    case PROP_PATTERN:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_string(value, cfgosrc->pattern);
        break;
    }
    case PROP_REQ_TYPES:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_string(value, cfgosrc->req_types);
    }
    case PROP_SUB_TIMEOUT:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_uint64(value, cfgosrc->sub_timeout);
        break;
    }
    case PROP_SUB_TRIES:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_int(value, cfgosrc->sub_tries);
        break;
    }
    case PROP_SUB_TRY_DELAY_INIT:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_uint64(value, cfgosrc->sub_try_delay_init);
        break;
    }
    case PROP_SUB_TRY_DELAY_STEP:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_uint(value, cfgosrc->sub_try_delay_step);
        break;
    }
    case PROP_SUB_TRY_DELAY_LEVEL:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_uint(value, cfgosrc->sub_try_delay_level);
        break;
    }
    case PROP_READ_TIMEOUT:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_uint64(value, cfgosrc->read_timeout);
        break;
    }
    case PROP_READ_TRIES:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_int(value, cfgosrc->read_tries);
        break;
    }
    case PROP_READ_TRY_DELAY_INIT:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_uint64(value, cfgosrc->read_try_delay_init);
        break;
    }
    case PROP_READ_TRY_DELAY_STEP:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_uint(value, cfgosrc->read_try_delay_step);
        break;
    }
    case PROP_READ_TRY_DELAY_LEVEL:
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        g_value_set_uint(value, cfgosrc->read_try_delay_level);
        break;
    }
    default:
        G_OBJECT_WARN_INVALID_PROPERTY_ID(object, property_id, pspec);
        break;
    }
}

void gst_cfgosrc_dispose(GObject *object)
{
    GstCfgoSrc *cfgosrc = GST_CFGOSRC(object);

    GST_DEBUG_OBJECT(cfgosrc, "dispose");
    {
        GST_CFGOSRC_LOCK_GUARD(cfgosrc);
        if (GST_CFGOSRC_PVS(cfgosrc)->task)
        {
            GST_CFGOSRC_PVS(cfgosrc)->task->detach();
            GST_CFGOSRC_PVS(cfgosrc)->task = nullptr;
        }
    }
    G_OBJECT_CLASS(gst_cfgosrc_parent_class)->dispose(object);
}

void gst_cfgosrc_finalize(GObject *object)
{
    GstCfgoSrc *cfgosrc = GST_CFGOSRC(object);

    GST_DEBUG_OBJECT(cfgosrc, "finalize");
    if (GST_CFGOSRC_PVS(cfgosrc))
    {
        delete(GST_CFGOSRC_PVS(cfgosrc));
    }
    if (cfgosrc->client_handle > 0)
    {
        cfgo_client_unref(cfgosrc->client_handle);
    }
    if (cfgosrc->pattern)
    {
        g_free(&cfgosrc->pattern);
    }
    if (cfgosrc->req_types)
    {
        g_free(&cfgosrc->req_types);
    }
    
    G_OBJECT_CLASS(gst_cfgosrc_parent_class)->finalize(object);
}

static GstStateChangeReturn
gst_cfgosrc_change_state(GstElement *element, GstStateChange transition)
{
    GstCfgoSrc *cfgosrc;
    GstStateChangeReturn ret;

    g_return_val_if_fail(GST_IS_CFGOSRC(element), GST_STATE_CHANGE_FAILURE);
    cfgosrc = GST_CFGOSRC(element);

    switch (transition)
    {
    case GST_STATE_CHANGE_NULL_TO_READY:
        break;
    case GST_STATE_CHANGE_READY_TO_PAUSED:
        break;
    case GST_STATE_CHANGE_PAUSED_TO_PLAYING:
        gst_cfgosrc_start(cfgosrc);
        break;
    default:
        break;
    }

    ret = GST_ELEMENT_CLASS (gst_cfgosrc_parent_class)->change_state(element, transition);

    switch (transition)
    {
    case GST_STATE_CHANGE_PLAYING_TO_PAUSED:
        gst_cfgosrc_stop(cfgosrc);
        break;
    case GST_STATE_CHANGE_PAUSED_TO_READY:
        break;
    case GST_STATE_CHANGE_READY_TO_NULL:
        break;
    default:
        break;
    }

    return ret;
}

static gboolean
plugin_init(GstPlugin *plugin)
{

    /* FIXME Remember to set the rank if it's an element that is meant
       to be autoplugged by decodebin. */
    return gst_element_register(plugin, "cfgosrc", GST_RANK_NONE,
                                GST_TYPE_CFGOSRC);
}

/* FIXME: these are normally defined by the GStreamer build system.
   If you are creating an element to be included in gst-plugins-*,
   remove these, as they're always defined.  Otherwise, edit as
   appropriate for your external plugin package. */
#ifndef VERSION
#define VERSION "0.0.FIXME"
#endif
#ifndef PACKAGE
#define PACKAGE "FIXME_package"
#endif
#ifndef PACKAGE_NAME
#define PACKAGE_NAME "FIXME_package_name"
#endif
#ifndef GST_PACKAGE_ORIGIN
#define GST_PACKAGE_ORIGIN "http://FIXME.org/"
#endif

GST_PLUGIN_DEFINE(GST_VERSION_MAJOR,
                  GST_VERSION_MINOR,
                  cfgosrc,
                  "FIXME plugin description",
                  plugin_init, VERSION, "LGPL", PACKAGE_NAME, GST_PACKAGE_ORIGIN)
