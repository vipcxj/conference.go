/* GStreamer
 * Copyright (C) 2024 FIXME <fixme@example.com>
 *
 * This library is free software; you can redistribute it and/or
 * modify it under the terms of the GNU Library General Public
 * License as published by the Free Software Foundation; either
 * version 2 of the License, or (at your option) any later version.
 *
 * This library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
 * Library General Public License for more details.
 *
 * You should have received a copy of the GNU Library General Public
 * License along with this library; if not, write to the
 * Free Software Foundation, Inc., 51 Franklin Street, Suite 500,
 * Boston, MA 02110-1335, USA.
 */
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
#include "cfgo/async.hpp"
#include "cfgo/capi.h"
#include "cfgo/cbridge.hpp"
#include "cfgo/client.hpp"
#include "cfgo/defer.hpp"
#include "asio.hpp"

GST_DEBUG_CATEGORY_STATIC(gst_cfgosrc_debug_category);
#define GST_CAT_DEFAULT gst_cfgosrc_debug_category

/* prototypes */

struct _GstCfgoSrcPrivate
{
    cfgo::close_chan_ptr sub_close_ch;
    cfgo::close_chan_ptr read_close_ch;
    asiochan::unbounded_channel<cfgo::Client::Ptr> client_ch;
    asiochan::unbounded_channel<std::string> pattern_ch;
    cfgo::AsyncMutex a_mutex;
    std::mutex mutex;
};

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
static gboolean gst_cfgosrc_send_event(GstElement *element, GstEvent *event);

enum
{
    PROP_0,
    PROP_CLIENT,
    PROP_PATTERN,
    PROP_SUB_TIMEOUT,
    PROP_READ_TIMEOUT
};

/* pad templates */

static GstStaticPadTemplate gst_cfgosrc_rtp_src_template =
    GST_STATIC_PAD_TEMPLATE("rtp_src_%u_%u",
                            GST_PAD_SRC,
                            GST_PAD_SOMETIMES,
                            GST_STATIC_CAPS("application/x-rtp"));

/* class initialization */

G_DEFINE_TYPE_WITH_CODE(GstCfgoSrc, gst_cfgosrc, GST_TYPE_BIN,
                        GST_DEBUG_CATEGORY_INIT(gst_cfgosrc_debug_category, "cfgosrc", 0,
                                                "debug category for cfgosrc element"));

auto _gst_cfgosrc_loop_task(
    GstCfgoSrc *cfgosrc, 
    cfgo::Client::Ptr client, 
    cfgo::close_chan_ptr sub_close_ch, 
    cfgo::close_chan_ptr read_close_ch
) -> asio::awaitable<void>
{
    if (co_await cfgosrc->priv->a_mutex.accquire())
    {
        DEFER({
            cfgosrc->priv->a_mutex.release();
        });

    }
    
}

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
    element_class->send_event = GST_DEBUG_FUNCPTR(gst_cfgosrc_send_event);

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
        gobject_class, PROP_SUB_TIMEOUT,
        g_param_spec_uint64(
            "sub-timeout", "sub-timeout", "timeout miliseconds of subscribing",
            0, G_MAXUINT64, 0,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
    g_object_class_install_property(
        gobject_class, PROP_READ_TIMEOUT,
        g_param_spec_uint64(
            "read-timeout", "read-timeout", "timeout miliseconds of reading data",
            0, G_MAXUINT64, 0,
            (GParamFlags)(G_PARAM_READWRITE | G_PARAM_STATIC_STRINGS)));
}

static void
gst_cfgosrc_init(GstCfgoSrc *cfgosrc)
{
    cfgosrc->priv = (GstCfgoSrcPrivate *)gst_cfgosrc_get_instance_private(cfgosrc);
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
            std::lock_guard lock(cfgosrc->priv->mutex);
            if (cfgosrc->client_handle > 0)
            {
                cfgo_client_unref(cfgosrc->client_handle);
            }
            cfgo_client_ref(handle);
            cfgosrc->client_handle = handle;
            GST_DEBUG_OBJECT(cfgosrc, "Client argument was changed to %d\n", handle);
        }
        else
        {
            GST_WARNING_OBJECT(cfgosrc, "Accept an invalid client handle: %d.\n", handle);
        }
        break;
    }
    case PROP_PATTERN:
    {
        std::lock_guard lock(cfgosrc->priv->mutex);
        cfgosrc->pattern = g_value_get_string(value);
        GST_DEBUG_OBJECT(cfgosrc, "Pattern argument was changed to %s\n", cfgosrc->pattern);
        break;
    }
    case PROP_SUB_TIMEOUT:
    {
        std::lock_guard lock(cfgosrc->priv->mutex);
        cfgosrc->sub_timeout = g_value_get_uint64(value);
        GST_DEBUG_OBJECT(cfgosrc, "Sub timeout argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->sub_timeout));
        break;
    }
    case PROP_READ_TIMEOUT:
    {
        std::lock_guard lock(cfgosrc->priv->mutex);
        cfgosrc->read_timeout = g_value_get_uint64(value);
        GST_DEBUG_OBJECT(cfgosrc, "Read timeout argument was changed to %" GST_TIME_FORMAT "\n", GST_TIME_ARGS(cfgosrc->read_timeout));
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
        std::lock_guard lock(cfgosrc->priv->mutex);
        g_value_set_int(value, cfgosrc->client_handle);
        break;
    }
    case PROP_PATTERN:
    {
        std::lock_guard lock(cfgosrc->priv->mutex);
        g_value_set_string(value, cfgosrc->pattern);
        break;
    }
    case PROP_SUB_TIMEOUT:
    {
        std::lock_guard lock(cfgosrc->priv->mutex);
        g_value_set_uint64(value, cfgosrc->sub_timeout);
        break;
    }
    case PROP_READ_TIMEOUT:
    {
        std::lock_guard lock(cfgosrc->priv->mutex);
        g_value_set_uint64(value, cfgosrc->read_timeout);
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

    /* clean up as possible.  may be called multiple times */

    G_OBJECT_CLASS(gst_cfgosrc_parent_class)->dispose(object);
}

void gst_cfgosrc_finalize(GObject *object)
{
    GstCfgoSrc *cfgosrc = GST_CFGOSRC(object);

    GST_DEBUG_OBJECT(cfgosrc, "finalize");

    /* clean up object here */

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
        break;
    default:
        break;
    }

    ret = GST_ELEMENT_CLASS (gst_cfgosrc_parent_class)->change_state(element, transition);

    switch (transition)
    {
    case GST_STATE_CHANGE_PLAYING_TO_PAUSED:
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
gst_cfgosrc_send_event(GstElement *element, GstEvent *event)
{

    return TRUE;
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
