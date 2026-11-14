import React, { useState } from 'react';
import { CodePanel, CodePanelProps } from './CodePanel';
import { CodeReaderModal } from './CodeReaderModal';

export function CodeBlock({
  title,
  readerFontSize,
  readerRange,
  ...props
}: CodePanelProps & {
  title?: string;
  readerFontSize?: number;
  readerRange?: string;
}) {
  const [reader, setReader] = useState<{
    source: string;
    hint?: string;
    partID: string;
    formatted: boolean;
    copySource?: string;
    copyLabel?: string;
    range?: string;
  } | null>(null);
  return (
    <>
      <CodePanel
        {...props}
        onExpand={state =>
          setReader({ source: props.source, hint: props.hint, copySource: props.copySource, copyLabel: props.copyLabel, range: readerRange, ...state })
        }
      />
      {reader && (
        <CodeReaderModal
          source={reader.source}
          hint={reader.hint}
          title={title || (props.lang === 'zh' ? '代码' : 'Code')}
          lang={props.lang || 'en'}
          fontSize={readerFontSize ?? props.fontSize ?? 13}
          dark={props.dark}
          initialPartID={reader.partID}
          initialFormatted={reader.formatted}
          copySource={reader.copySource}
          copyLabel={reader.copyLabel}
          range={reader.range}
          allowEmbedded={props.allowEmbedded}
          allowFormat={props.allowFormat}
          onClose={() => setReader(null)}
        />
      )}
    </>
  );
}
