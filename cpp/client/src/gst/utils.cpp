#include "cfgo/gst/utils.hpp"

namespace cfgo
{
    namespace gst
    {
        CFGO_DEFINE_MAKE_SHARED(gst_element, GstElement, gst_object_ref, gst_object_unref)
    } // namespace gst
    
} // namespace cfgo
