// Resolve actual JS styles with the same Yoga sources used by React Native.
#include <yoga/Yoga.h>
#include <cstdio>
#include <cstdlib>
#include <string>

static void width(YGNodeRef node, const char* value, bool maximum = false) {
  const bool percent = std::string(value).back() == '%';
  const float number = std::strtof(value, nullptr);
  if (maximum) {
    if (percent) YGNodeStyleSetMaxWidthPercent(node, number);
    else YGNodeStyleSetMaxWidth(node, number);
  } else {
    if (percent) YGNodeStyleSetWidthPercent(node, number);
    else YGNodeStyleSetWidth(node, number);
  }
}

int main(int argc, char** argv) {
  if (argc != 10) return 2;
  auto root = YGNodeNew();
  YGNodeStyleSetWidth(root, std::strtof(argv[1], nullptr));
  auto row = YGNodeNew();
  YGNodeStyleSetFlexDirection(row, YGFlexDirectionRow);
  YGNodeStyleSetJustifyContent(row, YGJustifyFlexEnd);
  YGNodeStyleSetAlignItems(row, YGAlignFlexStart);
  YGNodeStyleSetGap(row, YGGutterColumn, std::strtof(argv[9], nullptr));
  YGNodeInsertChild(root, row, 0);
  auto bubble = YGNodeNew();
  width(bubble, argv[2], true);
  YGNodeStyleSetPadding(bubble, YGEdgeHorizontal, std::strtof(argv[3], nullptr));
  YGNodeStyleSetPadding(bubble, YGEdgeVertical, std::strtof(argv[4], nullptr));
  YGNodeInsertChild(row, bubble, 0);
  auto markdown = YGNodeNew();
  YGNodeInsertChild(bubble, markdown, 0);
  auto range = YGNodeNew();
  width(range, argv[5]); width(range, argv[6], true);
  YGNodeInsertChild(markdown, range, 0);
  auto text = YGNodeNew();
  width(text, argv[8]);
  YGNodeStyleSetHeight(text, std::strtof(argv[7], nullptr));
  YGNodeInsertChild(range, text, 0);
  auto avatar = YGNodeNew();
  YGNodeStyleSetWidth(avatar, 26); YGNodeStyleSetHeight(avatar, 26);
  YGNodeInsertChild(row, avatar, 1);
  YGNodeCalculateLayout(root, YGUndefined, YGUndefined, YGDirectionLTR);
  std::printf("{\"bubbleWidth\":%f,\"textWidth\":%f,\"textHeight\":%f}\n",
              YGNodeLayoutGetWidth(bubble), YGNodeLayoutGetWidth(text), YGNodeLayoutGetHeight(text));
  YGNodeFreeRecursive(root);
}
