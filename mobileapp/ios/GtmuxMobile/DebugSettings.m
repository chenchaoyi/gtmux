#import <React/RCTBridgeModule.h>

@interface RCT_EXTERN_MODULE (DebugSettings, NSObject)

RCT_EXTERN_METHOD(record : (nonnull NSString *)line)
RCT_EXTERN_METHOD(reset)

@end
