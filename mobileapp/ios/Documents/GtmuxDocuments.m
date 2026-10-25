#import <React/RCTBridgeModule.h>

@interface RCT_EXTERN_MODULE (GtmuxDocuments, NSObject)
RCT_EXTERN_METHOD(open:(NSDictionary *)request resolver:(RCTPromiseResolveBlock)resolve rejecter:(RCTPromiseRejectBlock)reject)
@end
