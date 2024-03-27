#ifndef _GST_CFGOSRC_H_
#define _GST_CFGOSRC_H_

#include <gst/gst.h>
#include "cfgo/exports.h"

G_BEGIN_DECLS

#define GST_TYPE_CFGOSRC   (gst_cfgosrc_get_type())
#define GST_CFGOSRC(obj)   (G_TYPE_CHECK_INSTANCE_CAST((obj),GST_TYPE_CFGOSRC,GstCfgoSrc))
#define GST_CFGOSRC_CLASS(klass)   (G_TYPE_CHECK_CLASS_CAST((klass),GST_TYPE_CFGOSRC,GstCfgoSrcClass))
#define GST_IS_CFGOSRC(obj)   (G_TYPE_CHECK_INSTANCE_TYPE((obj),GST_TYPE_CFGOSRC))
#define GST_IS_CFGOSRC_CLASS(obj)   (G_TYPE_CHECK_CLASS_TYPE((klass),GST_TYPE_CFGOSRC))

typedef struct _GstCfgoSrc GstCfgoSrc;
typedef struct _GstCfgoSrcClass GstCfgoSrcClass;
typedef struct _GstCfgoSrcPrivate GstCfgoSrcPrivate;


struct _GstCfgoSrc
{
  GstBin bin;

  int client_handle;
  const gchar * pattern;
  guint64 sub_timeout;
  guint64 read_timeout;

  /*< private >*/
  GstCfgoSrcPrivate *priv;
};

struct _GstCfgoSrcClass
{
  GstBinClass parent_class;
};

GType gst_cfgosrc_get_type (void);

CFGO_API_WITHOUT_EXTERN GST_PLUGIN_STATIC_DECLARE (cfgosrc);

G_END_DECLS

#endif
