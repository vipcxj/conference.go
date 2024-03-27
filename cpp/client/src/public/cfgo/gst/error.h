#ifndef _CFGO_GST_ERROR_H_
#define _CFGO_GST_ERROR_H_

#include "glib.h"

G_BEGIN_DECLS

typedef enum
{
    CFGO_ERROR_TIMEOUT,
    CFGO_ERROR_GENERAL
} CfgoError;

#define CFGO_ERROR (cfgo_error_quark ())
GQuark cfgo_error_quark (void);

void cfgo_error_set_timeout (GError ** error);
const gchar * cfgo_error_get_trace (GError *error);
const gchar * cfgo_error_get_message (GError *error);

G_END_DECLS

#endif