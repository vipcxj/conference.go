#include "cfgo/utils.hpp"
#include <system_error>

namespace cfgo {

    std::string what(const std::exception_ptr &eptr)
    {
        if (!eptr) { throw std::bad_exception(); }

        try { std::rethrow_exception(eptr); }
        catch (const std::exception  &e)  { return e.what()   ; }
        catch (const std::string     &e)  { return e          ; }
        catch (const char            *e)  { return e          ; }
        catch (const std::error_code &e) { return e.message(); }
        catch (...)                      { return "who knows"; }
    }
}