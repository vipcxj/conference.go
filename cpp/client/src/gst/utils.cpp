#include "cfgo/gst/utils.hpp"

namespace cfgo
{
    namespace gst
    {
        CFGO_DEFINE_MAKE_SHARED(gst_element, GstElement, gst_object_ref, gst_object_unref)
        CFGO_DEFINE_MAKE_SHARED(gst_pad, GstPad, gst_object_ref, gst_object_unref)
        CFGO_DEFINE_STEAL_SHARED(g_error, GError, g_error_free)

        std::string get_pad_full_name(GstPad * pad)
        {
            std::string res = "pad ";
            auto element = gst_pad_get_parent_element(pad);
            if (element)
            {
                res = res + GST_PAD_NAME(pad) + " of " + GST_ELEMENT_NAME(element);
                gst_object_unref(element);
                return res;
            }
            else
            {
                return res + GST_PAD_NAME(pad);
            }
        }
    } // namespace gst
    
} // namespace cfgo
