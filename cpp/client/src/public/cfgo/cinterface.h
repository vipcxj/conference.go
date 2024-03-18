#ifndef _CFGO_CINTERFACE_HPP_
#define _CFGO_CINTERFACE_HPP_

#include "gst/gst.h"
#include "rtc/rtc.h"
#include "cfgo/exports.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef enum {
    ALL = 0,
    SOME = 1,
    NONE = 2,
    PUBLISH_ID = 3,
    STREAM_ID = 4,
    TRACK_ID = 5,
    TRACK_RID = 6,
    TRACK_LABEL_ALL_MATCH = 7,
    TRACK_LABEL_SOME_MATCH = 8,
    TRACK_LABEL_NONE_MATCH = 9,
    TRACK_LABEL_ALL_HAS = 10,
    TRACK_LABEL_SOME_HAS = 11,
    TRACK_LABEL_NONE_HAS = 12,
    TRACK_TYPE = 13
} cfgoPatternOp;

typedef enum
{
    CFGO_ERR_SUCCESS = 0,
    CFGO_ERR_FAILURE = -1
} cfgoErr;

typedef struct _cfgo_pattern
{
    cfgoPatternOp op;
    const char **args;
    _cfgo_pattern *children;
} cfgoPattern;

typedef struct
{
    const char * signal_url;
    const char * token;
    const rtcConfiguration * rtc_config;
    bool thread_safe;
    int execution_context_handle;
} cfgoConfiguration;

CFGO_API int cfgo_execution_context_create_io_context();
CFGO_API int cfgo_execution_context_create_thread_pool(int n);
CFGO_API int cfgo_execution_context_create_thread_pool_auto();
CFGO_API int cfgo_execution_context_release(int handle);

CFGO_API int cfgo_close_chan_create();
CFGO_API int cfgo_close_chan_close(int ctx_handle, int chan_handle);
CFGO_API int cfgo_close_chan_release(int handle);

CFGO_API int cfgo_client_create(const cfgoConfiguration * config);
CFGO_API int cfgo_client_release(int handle);
CFGO_API int cfgo_client_subscribe(int client_handle, const cfgoPattern * pattern, const char ** req_types, int asio_chan_handle);

CFGO_API const char * cfgo_subscribation_get_sub_id(int sub_handle);
CFGO_API const char * cfgo_subscribation_get_pub_id(int sub_handle);
CFGO_API int cfgo_subscribation_get_track_count(int sub_handle);
CFGO_API int cfgo_subscribation_get_track_at(int sub_handle, int index);
CFGO_API int cfgo_subscribation_release(int sub_handle);

CFGO_API const char * cfgo_track_get_type(int track_handle);
CFGO_API const char * cfgo_track_get_pub_id(int track_handle);
CFGO_API const char * cfgo_track_get_global_id(int track_handle);
CFGO_API const char * cfgo_track_get_bind_id(int track_handle);
CFGO_API const char * cfgo_track_get_rid(int track_handle);
CFGO_API const char * cfgo_track_get_stream_id(int track_handle);
CFGO_API int cfgo_track_get_label_count(int track_handle);
CFGO_API const char * cfgo_track_get_label_at(int track_handle, const char * name);
CFGO_API void * cfgo_track_get_gst_caps(int track_handle);
CFGO_API const unsigned char * cfgo_track_receive_msg(int track_handle);


#ifdef __cplusplus
}
#endif

#endif