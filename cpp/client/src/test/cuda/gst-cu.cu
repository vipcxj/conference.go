#include "gst-cu.cuh"
#include <cuda_fp16.h>

template<unsigned int channel>
__device__ inline void _copy_frame_(unsigned int width, unsigned int height, unsigned int channels, unsigned int frames, unsigned char * d_src, int stride, unsigned char * d_target, int frame, int index, int col, int row)
{
    _copy_frame_<channel - 1>(width, height, channels, frames, d_src, stride, d_target, frame, index, col, row);
    d_target[index * channels * frames * width * height + channel * frames * width * height + frame * width * height + row * width + col] = d_src[row * stride + col * channels + channel];
}

template<>
__device__ inline void _copy_frame_<0U>(unsigned int width, unsigned int height, unsigned int channels, unsigned int frames, unsigned char * d_src, int stride, unsigned char * d_target, int frame, int index, int col, int row)
{
    d_target[index * channels * frames * width * height + frame * width * height + row * width + col] = d_src[row * stride + col * channels];
}

template<unsigned int width, unsigned int height, unsigned int channels, unsigned int frames>
__global__ void _copy_frame(unsigned char * d_src, int stride, unsigned char * d_target, int frame, int index)
{
    int col = blockIdx.x * blockDim.x + threadIdx.x;
    int row = blockIdx.y * blockDim.y + threadIdx.y;
    if (col < width && row < height)
    {
        _copy_frame_<channels - 1>(width, height, channels, frames, d_src, stride, d_target, frame, index, col, row);
    }
}

template<unsigned int frame>
__device__ inline void _copy_ai_input_(unsigned int width, unsigned int height, unsigned int frames, unsigned char * d_src, int src_index, half * d_target, int tgt_index, int col, int row)
{
    _copy_ai_input_<frame - 1>(width, height, frames, d_src, src_index, d_target, tgt_index, col, row);
    d_target[tgt_index * 3 * frames * width * height + 0 * frames * width * height + frame * width * height + row * width + col] = __hmul(__hadd(__hdiv(__int2half_rn(d_src[src_index * 3 * frames * width * height + 0 * frames * width * height + frame * width * height + row * width + col]), 256), __float2half(-0.485f)), 4.36681f);
    d_target[tgt_index * 3 * frames * width * height + 1 * frames * width * height + frame * width * height + row * width + col] = __hmul(__hadd(__hdiv(__int2half_rn(d_src[src_index * 3 * frames * width * height + 1 * frames * width * height + frame * width * height + row * width + col]), 256), __float2half(-0.456f)), 4.464286f);
    d_target[tgt_index * 3 * frames * width * height + 2 * frames * width * height + frame * width * height + row * width + col] = __hmul(__hadd(__hdiv(__int2half_rn(d_src[src_index * 3 * frames * width * height + 2 * frames * width * height + frame * width * height + row * width + col]), 256), __float2half(-0.406f)), 4.444444f);
}

template<>
__device__ inline void _copy_ai_input_<0U>(unsigned int width, unsigned int height, unsigned int frames, unsigned char * d_src, int src_index, half * d_target, int tgt_index, int col, int row)
{
    d_target[tgt_index * 3 * frames * width * height + 0 * frames * width * height + row * width + col] = __hmul(__hadd(__hdiv(__int2half_rn(d_src[src_index * 3 * frames * width * height + 0 * frames * width * height + row * width + col]), 256), __float2half(-0.485f)), 4.36681f);
    d_target[tgt_index * 3 * frames * width * height + 1 * frames * width * height + row * width + col] = __hmul(__hadd(__hdiv(__int2half_rn(d_src[src_index * 3 * frames * width * height + 1 * frames * width * height + row * width + col]), 256), __float2half(-0.456f)), 4.464286f);
    d_target[tgt_index * 3 * frames * width * height + 2 * frames * width * height + row * width + col] = __hmul(__hadd(__hdiv(__int2half_rn(d_src[src_index * 3 * frames * width * height + 2 * frames * width * height + row * width + col]), 256), __float2half(-0.406f)), 4.444444f);
}

template<unsigned int width, unsigned int height, unsigned int frames>
__global__ void _copy_ai_input(unsigned char * d_src, int src_index, half * d_target, int tgt_index)
{
    int col = blockIdx.x * blockDim.x + threadIdx.x;
    int row = blockIdx.y * blockDim.y + threadIdx.y;
    if (col < width && row < height)
    {
        _copy_ai_input_<frames - 1>(width, height, frames, d_src, src_index, d_target, tgt_index, col, row);
    }
}

namespace cfgo
{
    namespace gst
    {
        void copy_frame(unsigned char * d_src, int stride, unsigned char * d_target, int frame, int index, CUstream stream)
        {
            _copy_frame<224, 224, 3, 16><<<{4, 4}, {8, 8}, 0, stream>>>(d_src, stride , d_target, frame, index);
        }
        void copy_ai_input(unsigned char * d_src, int src_index, half * d_target, int tgt_index, CUstream stream)
        {
            _copy_ai_input<224, 224, 16><<<{4, 4}, {8, 8}, 0, stream>>>(d_src, src_index, d_target, tgt_index);
        }
    } // namespace gst
    
} // namespace cfgo
