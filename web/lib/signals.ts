export function readBinaryPulse(text: string): number[] | undefined {
  if (!/^[+-]{512}\n$/.test(text)) return undefined;
  return Array.from(text.slice(0, 512), (sign) => (sign === "+" ? 1 : -1));
}

export function pulseStatistics(pulse: readonly number[]) {
  if (
    !pulse.length ||
    pulse.length > 4096 ||
    pulse.some((v) => v !== 1 && v !== -1)
  ) {
    throw new Error("A pulse must contain between 1 and 4096 binary signs.");
  }
  const correlations = Array.from({ length: pulse.length - 1 }, (_, offset) => {
    const lag = offset + 1;
    let sum = 0;
    for (let i = 0; i < pulse.length - lag; i++)
      sum += pulse[i] * pulse[i + lag];
    return sum;
  });
  const energy = correlations.reduce((sum, c) => sum + c * c, 0);
  return {
    correlations,
    energy,
    peak: Math.max(0, ...correlations.map(Math.abs)),
    merit: energy ? pulse.length ** 2 / (2 * energy) : null,
  };
}
