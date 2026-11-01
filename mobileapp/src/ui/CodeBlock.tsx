import React, { useState } from 'react';
import { CodePanel, CodePanelProps } from './CodePanel';
import { CodeReaderModal } from './CodeReaderModal';

export function CodeBlock({
  title,
  readerFontSize,
  ...props
}: CodePanelProps & {
  title?: string;
  readerFontSize?: number;
}) {
  const [reader, setReader] = useState<{
    source: string;
    hint?: string;
    partID: string;
    formatted: boolean;
  } | null>(null);
  return (
    <>
      <CodePanel
        {...props}
        onExpand={state =>
          setReader({ source: props.source, hint: props.hint, ...state })
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
          allowEmbedded={props.allowEmbedded}
          allowFormat={props.allowFormat}
          onClose={() => setReader(null)}
        />
      )}
    </>
  );
}
