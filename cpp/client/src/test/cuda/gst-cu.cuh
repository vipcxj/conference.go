#ifndef _CFGO_GST_CU_CUH_
#define _CFGO_GST_CU_CUH_

#include <cuda.h>
#include <cuda_fp16.h>

namespace cfgo
{
    namespace gst
    {
        void copy_frame(unsigned char * d_src, int stride, unsigned char * d_target, int frame, int index, CUstream stream);
        void copy_ai_input(unsigned char * d_src, int src_index, half * d_target, int tgt_index, CUstream stream);
    } // namespace gst
    
} // namespace cfgo

#endif