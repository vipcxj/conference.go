#ifndef _CFGO_GST_UTILS_H_
#define _CFGO_GST_UTILS_H_

#include "gst/gst.h"
#include <memory>

namespace cfgo
{
    #define CFGO_DECLARE_MAKE_SHARED(N, T) \
        using T ## SPtr = std::shared_ptr<T>; \
        T ## SPtr make_shared_ ## N(T *t);

    #define CFGO_DEFINE_MAKE_SHARED(N, T, REF, UNREF) \
        T ## SPtr make_shared_ ## N(T *t) \
        { \
            if (t) REF(t); \
            return std::shared_ptr<T>(t, [](T * t) { \
                if (t) UNREF(t); \
            }); \
        }   

    CFGO_DECLARE_MAKE_SHARED(gst_element, GstElement);

} // namespace cfgo


#endif