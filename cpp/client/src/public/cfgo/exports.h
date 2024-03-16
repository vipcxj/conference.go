#ifndef _CFGO_EXPORTS_H_
#define _CFGO_EXPORTS_H_

// Define EXPORTED for any platform
#if defined _WIN32 || defined __CYGWIN__
  #if defined BUILDING_CFGO || defined WIN_EXPORT
    // Exporting...
    #ifdef __GNUC__
      #define CFGO_API __attribute__ ((dllexport)) extern
    #else
      #define CFGO_API __declspec(dllexport) extern // Note: actually gcc seems to also supports this syntax.
    #endif
  #else
    #ifdef __GNUC__
      #define CFGO_API __attribute__ ((dllimport)) extern
    #else
      #define CFGO_API __declspec(dllimport) extern // Note: actually gcc seems to also supports this syntax.
    #endif
  #endif
  #define CFGO_HIDDEN_API extern
#else
  #if __GNUC__ >= 4
    #ifdef BUILDING_CFGO
      #define CFGO_API __attribute__ ((visibility ("default"))) extern
      #define CFGO_HIDDEN_API  __attribute__ ((visibility ("hidden"))) extern
    #else
      #define CFGO_API extern
      #define CFGO_HIDDEN_API extern
    #endif
  #else
    #define CFGO_API extern
    #define CFGO_HIDDEN_API extern
  #endif
#endif

#endif