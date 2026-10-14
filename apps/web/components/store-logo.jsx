"use client";

import Image from "next/image";
import { useState } from "react";

export default function StoreLogo({ name, hasLogo, src, className = "" }) {
  const [failed, setFailed] = useState(false);
  return (
    <div className={`logo-mark ${className}`}>
      {hasLogo && !failed ? (
        <Image
          unoptimized
          src={src}
          alt={`${name} logo`}
          width={96}
          height={96}
          onError={() => setFailed(true)}
        />
      ) : (
        <span aria-hidden="true">{name.slice(0, 1).toUpperCase()}</span>
      )}
    </div>
  );
}
