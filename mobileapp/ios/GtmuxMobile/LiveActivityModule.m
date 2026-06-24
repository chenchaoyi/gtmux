#import <React/RCTBridgeModule.h>

@interface RCT_EXTERN_MODULE (LiveActivityModule, NSObject)

RCT_EXTERN_METHOD(areEnabled : (RCTPromiseResolveBlock)resolve
                  rejecter : (RCTPromiseRejectBlock)reject)

RCT_EXTERN_METHOD(start : (nonnull NSNumber *)waiting
                  working : (nonnull NSNumber *)working
                  idle : (nonnull NSNumber *)idle
                  resolver : (RCTPromiseResolveBlock)resolve
                  rejecter : (RCTPromiseRejectBlock)reject)

RCT_EXTERN_METHOD(update : (nonnull NSNumber *)waiting
                  working : (nonnull NSNumber *)working
                  idle : (nonnull NSNumber *)idle)

RCT_EXTERN_METHOD(end)

@end
