#include "gst-cu.cuh"
#include <cuda_fp16.h>

__global__ void _copy_ai_input(unsigned char * d_src, int stride, half * d_target, int frame)
{
    int col = blockIdx.x * blockDim.x + threadIdx.x;
    int row = blockIdx.y * blockDim.y + threadIdx.y;
    if (col < 224 && row < 224)
    {
        half * half_target = (half *) d_target;
        half_target[0 * 16 * 224 * 224 + frame * 224 * 224 + row * 224 + col] = __hmul(__hadd(__hdiv(__int2half_rn(d_src[row * stride + col * 3]), 256), __float2half(-0.485f)), 4.36681f);
        half_target[1 * 16 * 224 * 224 + frame * 224 * 224 + row * 224 + col] = __hmul(__hadd(__hdiv(__int2half_rn(d_src[row * stride + col * 3 + 1]), 256), __float2half(-0.456f)), 4.464286f);
        half_target[2 * 16 * 224 * 224 + frame * 224 * 224 + row * 224 + col] = __hmul(__hadd(__hdiv(__int2half_rn(d_src[row * stride + col * 3 + 2]), 256), __float2half(-0.406f)), 4.444444f);
    }
}

namespace cfgo
{
    namespace gst
    {
        void copy_ai_input(unsigned char * d_src, int stride, half * d_target, int frame, CUstream stream)
        {
            _copy_ai_input<<<{4, 4}, {8, 8}, 0, stream>>>(d_src, stride, (half *) d_target, frame);
        }
    } // namespace gst
    
} // namespace cfgo
