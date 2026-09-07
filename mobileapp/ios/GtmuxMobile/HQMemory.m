#import <React/RCTBridgeModule.h>

@interface RCT_EXTERN_MODULE (HQMemory, NSObject)

RCT_EXTERN_METHOD(save
                  : (nonnull NSString *)url token
                  : (nonnull NSString *)token resolver
                  : (RCTPromiseResolveBlock)resolve rejecter
                  : (RCTPromiseRejectBlock)reject)
RCT_EXTERN_METHOD(state
                  : (RCTPromiseResolveBlock)resolve rejecter
                  : (RCTPromiseRejectBlock)reject)
RCT_EXTERN_METHOD(forget
                  : (RCTPromiseResolveBlock)resolve rejecter
                  : (RCTPromiseRejectBlock)reject)

@end
