#import <React/RCTBridgeModule.h>

@interface RCT_EXTERN_MODULE (UsageWidgetsModule, NSObject)
RCT_EXTERN_METHOD(configure:(NSString * _Nullable)selected resolver:(RCTPromiseResolveBlock)resolve rejecter:(RCTPromiseRejectBlock)reject)
@end
