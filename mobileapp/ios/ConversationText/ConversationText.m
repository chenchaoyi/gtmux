#import <React/RCTViewManager.h>

@interface RCT_EXTERN_MODULE (ConversationTextViewManager, RCTViewManager)
RCT_EXPORT_VIEW_PROPERTY(document, NSDictionary)
RCT_EXPORT_VIEW_PROPERTY(onSize, RCTDirectEventBlock)
RCT_EXPORT_VIEW_PROPERTY(onSelection, RCTDirectEventBlock)
RCT_EXPORT_VIEW_PROPERTY(onLink, RCTDirectEventBlock)
@end
