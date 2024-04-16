#include "cfgo/async_locker.hpp"
#include "cfgo/async.hpp"
#include <list>
#include <memory>
#include <algorithm>
#include <limits>

namespace cfgo
{
    namespace detail
    {
        class AsyncBlocker
        {
        public:
            AsyncBlocker(std::uint32_t id): m_id(id) {}
            AsyncBlocker(const AsyncBlocker &) = delete;
            AsyncBlocker & operator = (const AsyncBlocker &) = delete;
            auto request_block(close_chan closer) -> asio::awaitable<bool>;
            auto await_unblock() -> asio::awaitable<void>;
            void unblock();
            std::uint32_t id() const noexcept;
        private:
            std::uint32_t m_id;
            bool m_block = false;
            bool m_blocked = false;
            unique_void_chan m_request_chan;
            unique_void_chan m_response_chan;
        };

        // can not concurrent with unblock
        auto AsyncBlocker::request_block(close_chan closer) -> asio::awaitable<bool>
        {
            if (m_block)
            {
                throw cpptrace::logic_error("request_block could not be called when already requested block.");
            }
            
            m_block = true;
            do
            {
                if (!co_await chan_read<void>(m_response_chan, closer))
                {
                    unblock();
                    co_return false;
                }
                if (m_blocked)
                {
                    co_return true;
                }
            } while (true);
        }
        
        // can not concurrent with request_block
        void AsyncBlocker::unblock()
        {
            m_block = false;
            std::ignore = m_request_chan.try_write();
        }

        auto AsyncBlocker::await_unblock() -> asio::awaitable<void>
        {
            if (m_blocked)
            {
                throw cpptrace::logic_error("await_unblock should not be called when already blocked.");
            }
            
            if (!m_block)
            {
                co_return;
            }
            m_blocked = true;
            do
            {
                std::ignore = m_response_chan.try_write();
                co_await m_request_chan.read();
                if (!m_block)
                {
                    m_blocked = false;
                    co_return;
                }
                
            } while (true);
        }

        std::uint32_t AsyncBlocker::id() const noexcept
        {
            return m_id;
        }

        class AsyncBlockerManager : public std::enable_shared_from_this<AsyncBlockerManager>
        {
        public:
            using ScheduleConfigure = cfgo::AsyncBlockerManager::Configure;
            struct BlockerRequest
            {
                std::uint32_t m_id;
                int m_priority;
                unique_chan<AsyncBlockerPtr> m_chan;
            };
            
            struct BlockerInfo
            {
                AsyncBlockerPtr m_blocker;
                std::uint32_t m_epoch;
                int m_priority;
                bool m_used;
                bool m_valid;
            };

            AsyncBlockerManager(const ScheduleConfigure & configure);
            AsyncBlockerManager(const AsyncBlockerManager &) = delete;
            AsyncBlockerManager & operator = (const AsyncBlockerManager &) = delete;

            auto lock(close_chan closer) -> asio::awaitable<void>;
            void unlock();
            auto add_blocker(int priority, close_chan closer) -> asio::awaitable<AsyncBlockerPtr>;
            void remove_blocker(std::uint32_t id);
        protected:
            auto _select_and_request_block(int n, close_chan closer) -> asio::awaitable<int>;
            std::uint32_t _calc_batch() const noexcept
            {
                std::uint32_t n;
                if (m_conf.target_batch > 0)
                {
                    if (m_conf.target_batch > m_blockers.size())
                    {
                        n = std::max(m_conf.min_batch, (std::uint32_t) m_blockers.size());
                    }
                    else
                    {
                        n = m_conf.target_batch;
                    }
                }
                else if (m_conf.target_batch < 0)
                {
                    int _n = m_conf.target_batch + m_blockers.size();
                    n = (std::uint32_t) std::max(m_conf.target_batch + (std::int64_t) m_blockers.size(), (std::int64_t) m_conf.min_batch);
                }
                else
                {
                    n = m_conf.min_batch;
                }
                return n;
            }
        private:
            ScheduleConfigure m_conf;
            std::list<BlockerInfo> m_blockers;
            std::list<BlockerRequest> m_blocker_requests;
            std::uint32_t m_next_id = 0;
            std::uint32_t m_next_epoch = 0;
            bool m_locked = false;
            unique_void_chan m_ready_ch;
            std::mutex m_mutex;
        };
        
        AsyncBlockerManager::AsyncBlockerManager(const ScheduleConfigure & configure): m_conf(configure) {}
        auto AsyncBlockerManager::add_blocker(int priority, close_chan closer) -> asio::awaitable<AsyncBlockerPtr>
        {
            auto self = shared_from_this();
            AsyncBlockerPtr block_ptr = nullptr;
            std::uint32_t id;
            unique_chan<AsyncBlockerPtr> chan{};
            {
                std::lock_guard lk(m_mutex);
                bool not_enouth = _calc_batch() > m_blockers.size();
                if (!m_locked)
                {
                    block_ptr = std::make_shared<AsyncBlocker>(m_next_id++);
                    m_blockers.push_back({block_ptr, m_next_epoch++, priority, false, true});
                    if (not_enouth && _calc_batch() <= m_blockers.size())
                    {
                        chan_must_write(m_ready_ch);
                    }
                }
                else
                {
                    id = m_next_id++;
                    m_blocker_requests.push_back({id, priority, chan});
                }
            }
            if (block_ptr)
            {
                co_return block_ptr;
            }
            auto c_block_ptr = co_await chan_read<AsyncBlockerPtr>(chan, closer);
            if (c_block_ptr)
            {
                co_return c_block_ptr.value();
            }
            else
            {
                std::lock_guard lk(m_mutex);
                auto req_iter = std::find_if(m_blocker_requests.begin(), m_blocker_requests.end(), [id](const BlockerRequest & request) -> bool {
                    return request.m_id == id;
                });
                if (req_iter != m_blocker_requests.end())
                {
                    m_blocker_requests.erase(req_iter);
                    throw CancelError(closer);
                }
                auto iter = std::find_if(m_blockers.begin(), m_blockers.end(), [id](const BlockerInfo & info) -> bool {
                    return info.m_blocker->id() == id;
                });
                if (iter == m_blockers.end())
                {
                    throw cpptrace::logic_error(THIS_IS_IMPOSSIBLE);
                }
                co_return iter->m_blocker;
            }
        }

        void AsyncBlockerManager::remove_blocker(std::uint32_t id)
        {
            std::lock_guard lk(m_mutex);
            m_blocker_requests.erase(
                std::remove_if(m_blocker_requests.begin(), m_blocker_requests.end(), [id](const BlockerRequest & request) {
                    return request.m_id == id;
                }),
                m_blocker_requests.end()
            );
            if (!m_locked)
            {
                m_blockers.erase(
                    std::remove_if(m_blockers.begin(), m_blockers.end(), [id](const BlockerInfo & info) {
                        return info.m_blocker->id() == id;
                    }),
                    m_blockers.end()
                );
            }
            else
            {
                auto iter = std::find_if(m_blockers.begin(), m_blockers.end(), [id](const BlockerInfo & info) {
                    return info.m_blocker->id() == id;
                });
                if (iter != m_blockers.end())
                {
                    iter->m_valid = false;
                }
            }
        }

        auto AsyncBlockerManager::lock(close_chan closer) -> asio::awaitable<void>
        {
            if (m_locked)
            {
                throw cpptrace::runtime_error("Already locked.");
            }
            auto self = shared_from_this();
            std::uint32_t batch;
            do
            {
                {
                    std::lock_guard lk(m_mutex);
                    batch = _calc_batch();
                    if (batch <= m_blockers.size())
                    {
                        assert(batch > 0);
                        if (m_locked)
                        {
                            throw cpptrace::runtime_error("Already locked.");
                        }
                        m_locked = true;
                        break;
                    }
                }
                co_await chan_read_or_throw<void>(m_ready_ch, closer);
            } while (true);
            try
            {
                AsyncTasksAll<void> tasks(closer);
                {
                    std::lock_guard lk(m_mutex);
                    std::vector<std::reference_wrapper<BlockerInfo>> selects(m_blockers.begin(), m_blockers.end());
                    std::sort(selects.begin(), selects.end(), [](const BlockerInfo & b1, const BlockerInfo & b2) -> bool {
                        if (b1.m_epoch == b2.m_epoch)
                        {
                            if (b1.m_priority == b2.m_priority)
                            {
                                return b1.m_blocker->id() < b2.m_blocker->id();
                            }
                            else
                            {
                                return b1.m_priority > b2.m_priority;
                            }
                        }
                        else
                        {
                            return b1.m_epoch < b2.m_epoch;
                        }
                    });
                    BlockerInfo & blocker_with_max_epoch = *std::max_element(selects.begin(), selects.end(), [](const BlockerInfo & b1, const BlockerInfo & b2) -> bool {
                        return b1.m_epoch < b2.m_epoch;
                    });
                    std::uint32_t max_epoch = blocker_with_max_epoch.m_epoch;

                    std::uint32_t n = 0;
                    for (std::uint32_t i = 0; i < selects.size(); ++i)
                    {
                        BlockerInfo & blocker = selects[i];
                        if (blocker.m_epoch == max_epoch)
                        {
                            ++n;
                        }
                        if (n > batch)
                        {
                            break;
                        }
                        blocker.m_used = true;
                        tasks.add_task(fix_async_lambda([blocker = blocker.m_blocker](auto closer) -> asio::awaitable<void> {
                            if (!co_await blocker->request_block(closer))
                            {
                                throw CancelError(closer);   
                            }
                        }));
                    }
                }
                co_await tasks.await();
            }
            catch(...)
            {
                unlock();
                std::rethrow_exception(std::current_exception());
            }
        }
        void AsyncBlockerManager::unlock()
        {
            {
                std::lock_guard lk(m_mutex);
                if (!m_locked)
                {
                    throw cpptrace::runtime_error("Not locked yet.");
                }
                m_locked = false;
                for (auto && blocker : m_blockers)
                {
                    if (blocker.m_used)
                    {
                        blocker.m_used = false;
                        blocker.m_blocker->unblock();
                    }
                }
                for (auto it = m_blockers.begin(); it != m_blockers.end();)
                {
                    auto & blocker = *it;
                    if (!blocker.m_valid)
                    {
                        it = m_blockers.erase(it);
                    }
                    else
                    {
                        ++it;
                    }
                }
                for (auto && request : m_blocker_requests)
                {
                    auto block_ptr = std::make_shared<AsyncBlocker>(request.m_id);
                    m_blockers.push_back({block_ptr, m_next_epoch++, request.m_priority, false, true});
                    chan_must_write(request.m_chan, block_ptr);
                }
                m_blocker_requests.clear();
            }
        }


    } // namespace detail

    AsyncBlocker::AsyncBlocker(detail::AsyncBlockerPtr impl): ImplBy(impl) {}

    auto AsyncBlocker::request_block(close_chan closer) -> asio::awaitable<bool>
    {
        return impl()->request_block(std::move(closer));
    }

    auto AsyncBlocker::await_unblock() -> asio::awaitable<void>
    {
        return impl()->await_unblock();
    }

    void AsyncBlocker::unblock()
    {
        impl()->unblock();
    }

    std::uint32_t AsyncBlocker::id() const noexcept
    {
        return impl()->id();
    }

    void AsyncBlockerManager::Configure::validate() const
    {
        if (min_batch < 1 || (std::uint64_t) min_batch > (std::uint64_t) std::numeric_limits<std::int32_t>::max())
        {
            throw cpptrace::runtime_error(fmt::format("Invalid min_batch. The min_batch must be greater or equal than 1 and less or equal than {}.", std::numeric_limits<std::int32_t>::max()));
        }
        if (target_batch > 0 && target_batch < min_batch)
        {
            throw cpptrace::runtime_error("Invalid target_batch. The target_batch must greater than min_batch when it is positive.");
        }
    }

    AsyncBlockerManager::AsyncBlockerManager(const Configure & configure): ImplBy(configure) {}

    auto AsyncBlockerManager::lock(close_chan closer) -> asio::awaitable<void>
    {
        return impl()->lock(std::move(closer));
    }

    void AsyncBlockerManager::unlock()
    {
        impl()->unlock();
    }
    
    auto AsyncBlockerManager::add_blocker(int priority, close_chan closer) -> asio::awaitable<AsyncBlocker::Ptr>
    {
        auto block_ptr = co_await impl()->add_blocker(priority, closer);
        co_return std::make_shared<AsyncBlocker>(block_ptr);
    }

    void AsyncBlockerManager::remove_blocker(std::uint32_t id)
    {
        impl()->remove_blocker(id);
    }
} // namespace cfgo

