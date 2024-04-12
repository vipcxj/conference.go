#ifndef _CFGO_GST_CU_CUH_
#define _CFGO_GST_CU_CUH_

#include <cuda.h>
#include <cuda_fp16.h>

namespace cfgo
{
    namespace gst
    {
        void copy_ai_input(unsigned char * d_src, int stride, half * d_target, int frame, CUstream stream);
    } // namespace gst
    
} // namespace cfgo

#endif