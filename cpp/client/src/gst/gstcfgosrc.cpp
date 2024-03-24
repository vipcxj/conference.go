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

GST_DEBUG_CATEGORY_STATIC (gst_cfgosrc_debug_category);
#define GST_CAT_DEFAULT gst_cfgosrc_debug_category

/* prototypes */


static void gst_cfgosrc_set_property (GObject * object,
    guint property_id, const GValue * value, GParamSpec * pspec);
static void gst_cfgosrc_get_property (GObject * object,
    guint property_id, GValue * value, GParamSpec * pspec);
static void gst_cfgosrc_dispose (GObject * object);
static void gst_cfgosrc_finalize (GObject * object);

static GstPad *gst_cfgosrc_request_new_pad (GstElement * element,
    GstPadTemplate * templ, const gchar * name);
static void gst_cfgosrc_release_pad (GstElement * element, GstPad * pad);
static GstStateChangeReturn
gst_cfgosrc_change_state (GstElement * element, GstStateChange transition);
static gboolean gst_cfgosrc_send_event (GstElement * element, GstEvent * event);
static gboolean gst_cfgosrc_query (GstElement * element, GstQuery * query);

static GstCaps* gst_cfgosrc_sink_getcaps (GstPad *pad);
static gboolean gst_cfgosrc_sink_setcaps (GstPad *pad, GstCaps *caps);
static gboolean gst_cfgosrc_sink_acceptcaps (GstPad *pad, GstCaps *caps);
static void gst_cfgosrc_sink_fixatecaps (GstPad *pad, GstCaps *caps);
static gboolean gst_cfgosrc_sink_activate (GstPad *pad);
static gboolean gst_cfgosrc_sink_activatepush (GstPad *pad, gboolean active);
static gboolean gst_cfgosrc_sink_activatepull (GstPad *pad, gboolean active);
static GstPadLinkReturn gst_cfgosrc_sink_link (GstPad *pad, GstPad *peer);
static void gst_cfgosrc_sink_unlink (GstPad *pad);
static GstFlowReturn gst_cfgosrc_sink_chain (GstPad *pad, GstBuffer *buffer);
static GstFlowReturn gst_cfgosrc_sink_chainlist (GstPad *pad, GstBufferList *bufferlist);
static gboolean gst_cfgosrc_sink_event (GstPad *pad, GstEvent *event);
static gboolean gst_cfgosrc_sink_query (GstPad *pad, GstQuery *query);
static GstFlowReturn gst_cfgosrc_sink_bufferalloc (GstPad *pad, guint64 offset, guint size,
    GstCaps *caps, GstBuffer **buf);
static GstIterator * gst_cfgosrc_sink_iterintlink (GstPad *pad);


static GstCaps* gst_cfgosrc_src_getcaps (GstPad *pad);
static gboolean gst_cfgosrc_src_setcaps (GstPad *pad, GstCaps *caps);
static gboolean gst_cfgosrc_src_acceptcaps (GstPad *pad, GstCaps *caps);
static void gst_cfgosrc_src_fixatecaps (GstPad *pad, GstCaps *caps);
static gboolean gst_cfgosrc_src_activate (GstPad *pad);
static gboolean gst_cfgosrc_src_activatepush (GstPad *pad, gboolean active);
static gboolean gst_cfgosrc_src_activatepull (GstPad *pad, gboolean active);
static GstPadLinkReturn gst_cfgosrc_src_link (GstPad *pad, GstPad *peer);
static void gst_cfgosrc_src_unlink (GstPad *pad);
static GstFlowReturn gst_cfgosrc_src_getrange (GstPad *pad, guint64 offset, guint length,
    GstBuffer **buffer);
static gboolean gst_cfgosrc_src_event (GstPad *pad, GstEvent *event);
static gboolean gst_cfgosrc_src_query (GstPad *pad, GstQuery *query);
static GstIterator * gst_cfgosrc_src_iterintlink (GstPad *pad);


enum
{
  PROP_0
};

/* pad templates */

static GstStaticPadTemplate gst_cfgosrc_sink_template =
GST_STATIC_PAD_TEMPLATE ("sink",
    GST_PAD_SINK,
    GST_PAD_ALWAYS,
    GST_STATIC_CAPS ("application/unknown")
    );

static GstStaticPadTemplate gst_cfgosrc_src_template =
GST_STATIC_PAD_TEMPLATE ("src",
    GST_PAD_SRC,
    GST_PAD_ALWAYS,
    GST_STATIC_CAPS ("application/unknown")
    );


/* class initialization */

G_DEFINE_TYPE_WITH_CODE (GstCfgoSrc, gst_cfgosrc, GST_TYPE_BIN,
  GST_DEBUG_CATEGORY_INIT (gst_cfgosrc_debug_category, "cfgosrc", 0,
  "debug category for cfgosrc element"));

static void
gst_cfgosrc_class_init (GstCfgoSrcClass * klass)
{
  GObjectClass *gobject_class = G_OBJECT_CLASS (klass);
  GstElementClass *element_class = GST_ELEMENT_CLASS (klass);

  /* Setting up pads and setting metadata should be moved to
     base_class_init if you intend to subclass this class. */
  gst_element_class_add_static_pad_template (element_class,
      &gst_cfgosrc_sink_template);
  gst_element_class_add_static_pad_template (element_class,
      &gst_cfgosrc_src_template);

  gst_element_class_set_static_metadata (GST_ELEMENT_CLASS(klass),
      "FIXME Long name", "Generic", "FIXME Description",
      "FIXME <fixme@example.com>");

  gobject_class->set_property = gst_cfgosrc_set_property;
  gobject_class->get_property = gst_cfgosrc_get_property;
  gobject_class->dispose = gst_cfgosrc_dispose;
  gobject_class->finalize = gst_cfgosrc_finalize;
  element_class->request_new_pad = GST_DEBUG_FUNCPTR (gst_cfgosrc_request_new_pad);
  element_class->release_pad = GST_DEBUG_FUNCPTR (gst_cfgosrc_release_pad);
  element_class->change_state = GST_DEBUG_FUNCPTR (gst_cfgosrc_change_state);
  element_class->send_event = GST_DEBUG_FUNCPTR (gst_cfgosrc_send_event);
  element_class->query = GST_DEBUG_FUNCPTR (gst_cfgosrc_query);

}

static void
gst_cfgosrc_init (GstCfgoSrc *cfgosrc)
{
  cfgosrc->priv = (GstCfgoSrcPrivate *) gst_cfgosrc_get_instance_private (cfgosrc);

  cfgosrc->sinkpad = gst_pad_new_from_static_template (&gst_cfgosrc_sink_template
      ,     
            "sink");
  gst_pad_set_getcaps_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_getcaps));
  gst_pad_set_setcaps_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_setcaps));
  gst_pad_set_acceptcaps_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_acceptcaps));
  gst_pad_set_fixatecaps_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_fixatecaps));
  gst_pad_set_activate_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_activate));
  gst_pad_set_activatepush_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_activatepush));
  gst_pad_set_activatepull_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_activatepull));
  gst_pad_set_link_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_link));
  gst_pad_set_unlink_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_unlink));
  gst_pad_set_chain_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_chain));
  gst_pad_set_chain_list_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_chainlist));
  gst_pad_set_event_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_event));
  gst_pad_set_query_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_query));
  gst_pad_set_bufferalloc_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_bufferalloc));
  gst_pad_set_iterate_internal_links_function (cfgosrc->sinkpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_sink_iterintlink));
  gst_element_add_pad (GST_ELEMENT(cfgosrc), cfgosrc->sinkpad);



  cfgosrc->srcpad = gst_pad_new_from_static_template (&gst_cfgosrc_src_template
      ,     
            "src");
  gst_pad_set_getcaps_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_getcaps));
  gst_pad_set_setcaps_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_setcaps));
  gst_pad_set_acceptcaps_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_acceptcaps));
  gst_pad_set_fixatecaps_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_fixatecaps));
  gst_pad_set_activate_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_activate));
  gst_pad_set_activatepush_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_activatepush));
  gst_pad_set_activatepull_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_activatepull));
  gst_pad_set_link_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_link));
  gst_pad_set_unlink_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_unlink));
  gst_pad_set_getrange_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_getrange));
  gst_pad_set_event_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_event));
  gst_pad_set_query_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_query));
  gst_pad_set_iterate_internal_links_function (cfgosrc->srcpad,
            GST_DEBUG_FUNCPTR(gst_cfgosrc_src_iterintlink));
  gst_element_add_pad (GST_ELEMENT(cfgosrc), cfgosrc->srcpad);


}

void
gst_cfgosrc_set_property (GObject * object, guint property_id,
    const GValue * value, GParamSpec * pspec)
{
  GstCfgoSrc *cfgosrc = GST_CFGOSRC (object);

  GST_DEBUG_OBJECT (cfgosrc, "set_property");

  switch (property_id) {
    default:
      G_OBJECT_WARN_INVALID_PROPERTY_ID (object, property_id, pspec);
      break;
  }
}

void
gst_cfgosrc_get_property (GObject * object, guint property_id,
    GValue * value, GParamSpec * pspec)
{
  GstCfgoSrc *cfgosrc = GST_CFGOSRC (object);

  GST_DEBUG_OBJECT (cfgosrc, "get_property");

  switch (property_id) {
    default:
      G_OBJECT_WARN_INVALID_PROPERTY_ID (object, property_id, pspec);
      break;
  }
}

void
gst_cfgosrc_dispose (GObject * object)
{
  GstCfgoSrc *cfgosrc = GST_CFGOSRC (object);

  GST_DEBUG_OBJECT (cfgosrc, "dispose");

  /* clean up as possible.  may be called multiple times */

  G_OBJECT_CLASS (gst_cfgosrc_parent_class)->dispose (object);
}

void
gst_cfgosrc_finalize (GObject * object)
{
  GstCfgoSrc *cfgosrc = GST_CFGOSRC (object);

  GST_DEBUG_OBJECT (cfgosrc, "finalize");

  /* clean up object here */

  G_OBJECT_CLASS (gst_cfgosrc_parent_class)->finalize (object);
}



static GstPad *
gst_cfgosrc_request_new_pad (GstElement * element, GstPadTemplate * templ,
    const gchar * name)
{

  return NULL;
}

static void
gst_cfgosrc_release_pad (GstElement * element, GstPad * pad)
{

}

static GstStateChangeReturn
gst_cfgosrc_change_state (GstElement * element, GstStateChange transition)
{
  GstCfgoSrc *cfgosrc;
  GstStateChangeReturn ret;

  g_return_val_if_fail (GST_IS_CFGOSRC (element), GST_STATE_CHANGE_FAILURE);
  cfgosrc = GST_CFGOSRC (element);

  switch (transition) {
    case GST_STATE_CHANGE_NULL_TO_READY:
      break;
    case GST_STATE_CHANGE_READY_TO_PAUSED:
      break;
    case GST_STATE_CHANGE_PAUSED_TO_PLAYING:
      break;
    default:
      break;
  }

  ret = GST_ELEMENT_CLASS (parent_class)->change_state (element, transition);

  switch (transition) {
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
gst_cfgosrc_send_event (GstElement * element, GstEvent * event)
{

  return TRUE;
}

static gboolean
gst_cfgosrc_query (GstElement * element, GstQuery * query)
{
  GstCfgoSrc *cfgosrc = GST_CFGOSRC (element);
  gboolean ret;

  GST_DEBUG_OBJECT (cfgosrc, "query");

  switch (GST_QUERY_TYPE (query)) {
    default:
      ret = GST_ELEMENT_CLASS (parent_class)->query (element, query);
      break;
  }

  return ret;
}

static GstCaps*
gst_cfgosrc_sink_getcaps (GstPad *pad)
{
  GstCfgoSrc *cfgosrc;
  GstCaps *caps;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "getcaps");

  caps = gst_caps_copy (gst_pad_get_pad_template_caps (pad));

  gst_object_unref (cfgosrc);
  return caps;
}

static gboolean
gst_cfgosrc_sink_setcaps (GstPad *pad, GstCaps *caps)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "setcaps");


  gst_object_unref (cfgosrc);
  return TRUE;
}

static gboolean
gst_cfgosrc_sink_acceptcaps (GstPad *pad, GstCaps *caps)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "acceptcaps");


  gst_object_unref (cfgosrc);
  return TRUE;
}

static void
gst_cfgosrc_sink_fixatecaps (GstPad *pad, GstCaps *caps)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "fixatecaps");


  gst_object_unref (cfgosrc);
}

static gboolean
gst_cfgosrc_sink_activate (GstPad *pad)
{
  GstCfgoSrc *cfgosrc;
  gboolean ret;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "activate");

  if (gst_pad_check_pull_range (pad)) {
    GST_DEBUG_OBJECT (pad, "activating pull");
    ret = gst_pad_activate_pull (pad, TRUE);
  } else {
    GST_DEBUG_OBJECT (pad, "activating push");
    ret = gst_pad_activate_push (pad, TRUE);
  }

  gst_object_unref (cfgosrc);
  return ret;
}

static gboolean
gst_cfgosrc_sink_activatepush (GstPad *pad, gboolean active)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "activatepush");


  gst_object_unref (cfgosrc);
  return TRUE;
}

static gboolean
gst_cfgosrc_sink_activatepull (GstPad *pad, gboolean active)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "activatepull");


  gst_object_unref (cfgosrc);
  return TRUE;
}

static GstPadLinkReturn
gst_cfgosrc_sink_link (GstPad *pad, GstPad *peer)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "link");


  gst_object_unref (cfgosrc);
  return GST_PAD_LINK_OK;
}

static void
gst_cfgosrc_sink_unlink (GstPad *pad)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "unlink");


  gst_object_unref (cfgosrc);
}

static GstFlowReturn
gst_cfgosrc_sink_chain (GstPad *pad, GstBuffer *buffer)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "chain");


  gst_object_unref (cfgosrc);
  return GST_FLOW_OK;
}

static GstFlowReturn
gst_cfgosrc_sink_chainlist (GstPad *pad, GstBufferList *bufferlist)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "chainlist");


  gst_object_unref (cfgosrc);
  return GST_FLOW_OK;
}

static gboolean
gst_cfgosrc_sink_event (GstPad *pad, GstEvent *event)
{
  gboolean res;
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "event");

  switch (GST_EVENT_TYPE (event)) {
    default:
      res = gst_pad_event_default (pad, event);
      break;
  }

  gst_object_unref (cfgosrc);
  return res;
}

static gboolean
gst_cfgosrc_sink_query (GstPad *pad, GstQuery *query)
{
  gboolean res;
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "query");

  switch (GST_QUERY_TYPE(query)) {
    default:
      res = gst_pad_query_default (pad, query);
      break;
  }

  gst_object_unref (cfgosrc);
  return res;
}

static GstFlowReturn
gst_cfgosrc_sink_bufferalloc (GstPad *pad, guint64 offset, guint size,
    GstCaps *caps, GstBuffer **buf)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "bufferalloc");


  *buf = gst_buffer_new_and_alloc (size);
  gst_buffer_set_caps (*buf, caps);

  gst_object_unref (cfgosrc);
  return GST_FLOW_OK;
}

static GstIterator *
gst_cfgosrc_sink_iterintlink (GstPad *pad)
{
  GstCfgoSrc *cfgosrc;
  GstIterator *iter;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "iterintlink");

  iter = gst_pad_iterate_internal_links_default (pad);

  gst_object_unref (cfgosrc);
  return iter;
}


static GstCaps*
gst_cfgosrc_src_getcaps (GstPad *pad)
{
  GstCfgoSrc *cfgosrc;
  GstCaps *caps;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "getcaps");

  caps = gst_pad_get_pad_template_caps (pad);

  gst_object_unref (cfgosrc);
  return caps;
}

static gboolean
gst_cfgosrc_src_setcaps (GstPad *pad, GstCaps *caps)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "setcaps");


  gst_object_unref (cfgosrc);
  return TRUE;
}

static gboolean
gst_cfgosrc_src_acceptcaps (GstPad *pad, GstCaps *caps)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "acceptcaps");


  gst_object_unref (cfgosrc);
  return TRUE;
}

static void
gst_cfgosrc_src_fixatecaps (GstPad *pad, GstCaps *caps)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "fixatecaps");


  gst_object_unref (cfgosrc);
}

static gboolean
gst_cfgosrc_src_activate (GstPad *pad)
{
  GstCfgoSrc *cfgosrc;
  gboolean ret;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "activate");

  if (gst_pad_check_pull_range (pad)) {
    GST_DEBUG_OBJECT (pad, "activating pull");
    ret = gst_pad_activate_pull (pad, TRUE);
  } else {
    GST_DEBUG_OBJECT (pad, "activating push");
    ret = gst_pad_activate_push (pad, TRUE);
  }

  gst_object_unref (cfgosrc);
  return ret;
}

static gboolean
gst_cfgosrc_src_activatepush (GstPad *pad, gboolean active)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "activatepush");


  gst_object_unref (cfgosrc);
  return TRUE;
}

static gboolean
gst_cfgosrc_src_activatepull (GstPad *pad, gboolean active)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "activatepull");


  gst_object_unref (cfgosrc);
  return TRUE;
}

static GstPadLinkReturn
gst_cfgosrc_src_link (GstPad *pad, GstPad *peer)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "link");


  gst_object_unref (cfgosrc);
  return GST_PAD_LINK_OK;
}

static void
gst_cfgosrc_src_unlink (GstPad *pad)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "unlink");


  gst_object_unref (cfgosrc);
}

static GstFlowReturn
gst_cfgosrc_src_getrange (GstPad *pad, guint64 offset, guint length,
    GstBuffer **buffer)
{
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "getrange");


  gst_object_unref (cfgosrc);
  return GST_FLOW_OK;
}

static gboolean
gst_cfgosrc_src_event (GstPad *pad, GstEvent *event)
{
  gboolean res;
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "event");

  switch (GST_EVENT_TYPE (event)) {
    default:
      res = gst_pad_event_default (pad, event);
      break;
  }

  gst_object_unref (cfgosrc);
  return res;
}

static gboolean
gst_cfgosrc_src_query (GstPad *pad, GstQuery *query)
{
  gboolean res;
  GstCfgoSrc *cfgosrc;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "query");

  switch (GST_QUERY_TYPE(query)) {
    default:
      res = gst_pad_query_default (pad, query);
      break;
  }

  gst_object_unref (cfgosrc);
  return res;
}

static GstIterator *
gst_cfgosrc_src_iterintlink (GstPad *pad)
{
  GstCfgoSrc *cfgosrc;
  GstIterator *iter;

  cfgosrc = GST_CFGOSRC (gst_pad_get_parent (pad));

  GST_DEBUG_OBJECT(cfgosrc, "iterintlink");

  iter = gst_pad_iterate_internal_links_default (pad);

  gst_object_unref (cfgosrc);
  return iter;
}


static gboolean
plugin_init (GstPlugin * plugin)
{

  /* FIXME Remember to set the rank if it's an element that is meant
     to be autoplugged by decodebin. */
  return gst_element_register (plugin, "cfgosrc", GST_RANK_NONE,
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

GST_PLUGIN_DEFINE (GST_VERSION_MAJOR,
    GST_VERSION_MINOR,
    cfgosrc,
    "FIXME plugin description",
    plugin_init, VERSION, "LGPL", PACKAGE_NAME, GST_PACKAGE_ORIGIN)

