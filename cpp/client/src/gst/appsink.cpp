#include "cfgo/gst/appsink.hpp"
#include "boost/circular_buffer.hpp"

#include <cstdint>
#include <limits>

namespace cfgo
{
    namespace gst
    {
        namespace detail
        {
            class AppSink : public std::enable_shared_from_this<AppSink>
            {
            public:
                using SampleBuffer = boost::circular_buffer<std::pair<std::uint32_t, GstSampleSPtr>>;
                AppSink(GstAppSink * sink);
                ~AppSink();

                auto pull_sample(close_chan closer) -> asio::awaitable<GstSampleSPtr>;
            private:
                GstAppSink * m_sink;
                SampleBuffer m_cache;
                unique_void_chan m_sample_notify;
                unique_void_chan m_eos_notify;
                std::mutex m_mutex;
                std::uint32_t m_seq;
                bool m_eos;

                static void on_eos(GstAppSink *appsink, gpointer userdata);
                static GstFlowReturn on_new_preroll(GstAppSink *appsink, gpointer userdata);
                static GstFlowReturn on_new_sample(GstAppSink *appsink, gpointer userdata);
                static gboolean on_new_event(GstAppSink *appsink, gpointer userdata);
                static gboolean on_propose_allocation(GstAppSink *appsink, GstQuery *query, gpointer userdata);

                std::uint32_t _makesure_min_seq() const noexcept;
            };
            
            AppSink::AppSink(GstAppSink * sink): m_sink(sink), m_seq(0), m_eos(false)
            {
                gst_object_ref(m_sink);
                GstAppSinkCallbacks callbacks{};
                callbacks.eos = &AppSink::on_eos;
                callbacks.new_preroll = &AppSink::on_new_preroll;
                callbacks.new_sample = &AppSink::on_new_sample;
                callbacks.new_event = &AppSink::on_new_event;
                callbacks.propose_allocation = &AppSink::on_propose_allocation;
                gst_app_sink_set_callbacks(m_sink, &callbacks, this, NULL);
            }
            
            AppSink::~AppSink()
            {
                GstAppSinkCallbacks callbacks {};
                gst_app_sink_set_callbacks(m_sink, &callbacks, NULL, NULL);
                gst_object_unref(m_sink);
            }

            void AppSink::on_eos(GstAppSink *appsink, gpointer userdata)
            {
                AppSink * self = (AppSink *) userdata;
                std::lock_guard lk(self->m_mutex);
                self->m_eos = true;
                chan_maybe_write(self->m_eos_notify);
            }
            GstFlowReturn AppSink::on_new_preroll(GstAppSink *appsink, gpointer userdata)
            {
                return GST_FLOW_OK;
            }
            GstFlowReturn AppSink::on_new_sample(GstAppSink *appsink, gpointer userdata)
            {
                AppSink * self = (AppSink *) userdata;
                std::lock_guard lk(self->m_mutex);
                if (self->m_seq == std::numeric_limits<std::uint32_t>::max())
                {
                    auto offset = self->_makesure_min_seq();
                    for (auto && v : self->m_cache)
                    {
                        v.first -= offset;
                    }
                    self->m_seq -= offset;
                }
                auto sample = gst_app_sink_pull_sample(appsink);
                if (sample)
                {
                    self->m_cache.push_back(std::make_pair(self->m_seq++, steal_shared_gst_sample(sample)));
                    chan_maybe_write(self->m_sample_notify);
                }
                return GST_FLOW_OK;
            }
            gboolean AppSink::on_new_event(GstAppSink *appsink, gpointer userdata)
            {
                return FALSE;
            }
            gboolean AppSink::on_propose_allocation(GstAppSink *appsink, GstQuery *query, gpointer userdata)
            {
                return FALSE;
            }

            std::uint32_t AppSink::_makesure_min_seq() const noexcept
            {
                if (m_cache.empty())
                {
                    return std::numeric_limits<std::uint32_t>::max();
                }
                else
                {
                    return m_cache.front().first;
                }
            }

            // only support one receiver at same time.
            auto AppSink::pull_sample(close_chan closer) -> asio::awaitable<GstSampleSPtr>
            {
                auto self = shared_from_this();
                GstSampleSPtr sample_ptr = nullptr;
                bool done = false;
                {
                    std::lock_guard lk(m_mutex);
                    if (!m_cache.empty())
                    {
                        sample_ptr = m_cache.front().second;
                        m_cache.pop_front();
                        done = true;
                    }
                    else if (m_eos)
                    {
                        done = true;
                    }
                }
                if (done)
                {
                    co_return sample_ptr;
                }
                co_await select_or_throw(closer, asiochan::ops::read(m_sample_notify, m_eos_notify));
                {
                    std::lock_guard lk(m_mutex);
                    if (m_cache.empty() && !m_eos)
                    {
                        throw cpptrace::logic_error("This should not happened. Only one receiver is supported at the same time.");
                    }
                    if (!m_cache.empty())
                    {
                        sample_ptr = m_cache.front().second;
                        m_cache.pop_front();
                    }
                }
                co_return sample_ptr;
            }

        } // namespace detail

        AppSink::AppSink(GstAppSink *sink) : ImplBy(sink) {}

        auto AppSink::pull_sample(close_chan closer) -> asio::awaitable<GstSampleSPtr>
        {
            return impl()->pull_sample(std::move(closer));
        }

    } // namespace gst
    
} // namespace cfgo
