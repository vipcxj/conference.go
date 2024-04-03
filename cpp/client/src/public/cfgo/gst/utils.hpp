#ifndef _CFGO_GST_UTILS_H_
#define _CFGO_GST_UTILS_H_

#include "gst/gst.h"
#include <memory>
#include <string>

namespace cfgo
{
    #define CFGO_DECLARE_MAKE_SHARED(N, T) \
        using T ## SPtr = std::shared_ptr<T>; \
        T ## SPtr make_shared_ ## N(T *t);

    #define CFGO_DECLARE_STEAL_SHARED(N, T) \
        using T ## SPtr = std::shared_ptr<T>; \
        T ## SPtr steal_shared_ ## N(T *t);

    #define CFGO_DEFINE_MAKE_SHARED(N, T, REF, UNREF) \
        T ## SPtr make_shared_ ## N(T *t) \
        { \
            if (t) REF(t); \
            return std::shared_ptr<T>(t, [](T * t) { \
                if (t) UNREF(t); \
            }); \
        }

    #define CFGO_DEFINE_STEAL_SHARED(N, T, FREE) \
        T ## SPtr steal_shared_ ## N(T *t) \
        { \
            return std::shared_ptr<T>(t, [](T * t) { \
                if (t) FREE(t); \
            }); \
        }   

    namespace gst
    {
        CFGO_DECLARE_MAKE_SHARED(gst_element, GstElement);
        CFGO_DECLARE_MAKE_SHARED(gst_pad, GstPad);
        CFGO_DECLARE_STEAL_SHARED(g_error, GError);

        std::string get_pad_full_name(GstPad * pad);
    } // namespace gst
} // namespace cfgo


#endif