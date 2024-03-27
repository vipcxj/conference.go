#include "cfgo/gst/utils.hpp"

namespace cfgo
{
    namespace gst
    {
        CFGO_DEFINE_MAKE_SHARED(gst_element, GstElement, gst_object_ref, gst_object_unref)
        CFGO_DEFINE_STEAL_SHARED(g_error, GError, g_error_free)
    } // namespace gst
    
} // namespace cfgo
